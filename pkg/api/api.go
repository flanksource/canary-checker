package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	vapi "github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
	prometheusapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

type Response struct {
	ServerName string     `json:"serverName"`
	Checks     pkg.Checks `json:"checks"`
}

var ServerName string

func Handler(w http.ResponseWriter, req *http.Request) {
	apiResponse := &Response{
		ServerName: ServerName,
		Checks:     cache.GetChecks(),
	}
	jsonData, err := json.Marshal(apiResponse)
	if err != nil {
		logger.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
		return
	}
	w.Write(jsonData)
}

func triggerCheckOnServer(serverUrl string, triggerData TriggerData) (*vapi.Response, error) {
	url := fmt.Sprintf("%s/api/triggerCheck", serverUrl)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	jsonData, err := json.Marshal(triggerData)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get url %s", url)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response body for url %s", url)
	}
	apiResponse := &vapi.Response{}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal json body for url %s", url)
	}
	return apiResponse, nil
}

type TriggerData struct {
	CheckKey    string `json:"checkKey"`
	CheckServer string `json:"server"`
}

func TriggerCheckHandler(w http.ResponseWriter, req *http.Request) {
	var td TriggerData
	err := json.NewDecoder(req.Body).Decode(&td)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsedServer := strings.Split(td.CheckServer, "@")
	serverURL := parsedServer[1]
	serverName := parsedServer[0]
	if serverURL == "local" {
		var check *pkg.Check
		data := cache.GetChecks()
		for _, _c := range data {
			c := _c
			if c.Key == td.CheckKey {
				check = &c
			}
		}

		if check == nil {
			http.Error(w, "No check found", http.StatusNotFound)
			return
		}

		var checker checks.Checker
		for _, _c := range checks.All {
			c := _c
			if c.Type() == check.Type {
				checker = c
			}
		}

		if checker == nil {
			http.Error(w, "No checker found", http.StatusNotFound)
			return
		}

		conf := cache.GetConfig(td.CheckKey)
		if conf == nil {
			http.Error(w, "Check config not found", http.StatusNotFound)
			return
		}
		result := checker.Check(conf)
		cache.AddCheck(*check.CheckCanary, result)
		metrics.Record(*check.CheckCanary, result)
	} else {
		td.CheckServer = fmt.Sprintf("%s@local", serverName)
		_, err := triggerCheckOnServer(serverURL, td)
		if err != nil {
			http.Error(w,
				fmt.Sprintf("Failed to trigger check on server %s: %v", serverURL, err),
				http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

type PrometheusGraphData struct {
	CheckType string `json:"checkType"`
	CheckKey  string `json:"checkKey"`
	Timeframe int    `json:"timeframe"`
}

type Timeseries struct {
	Time  float64 `json:"time"`
	Value string  `json:"value"`
}

func PrometheusGraphHandler(prometheusHost string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var pg PrometheusGraphData
		err := json.NewDecoder(req.Body).Decode(&pg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		timeframe := time.Duration(pg.Timeframe) * time.Second
		if timeframe == 0 {
			timeframe = 3600 * time.Second
		}

		transportConfig := prometheusapi.DefaultRoundTripper.(*http.Transport)
		transportConfig.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		client, err := prometheusapi.NewClient(prometheusapi.Config{
			Address:      prometheusHost,
			RoundTripper: transportConfig,
		})
		if err != nil {
			log.Errorf("Failed to create prometheus client: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		v1api := v1.NewAPI(client)

		canarySuccessCount, err := getCanarySuccess(v1api, "http", "https://httpstat.us/301", timeframe)
		if err != nil {
			log.Errorf("Failed to get canary success count: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		canaryFailedCount, err := getCanaryFailed(v1api, "http", "https://httpstat.us/301", timeframe)
		if err != nil {
			log.Errorf("Failed to get canary success count: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		canaryLatency, err := getCanaryLatency(v1api, "http", "https://httpstat.us/301", timeframe)
		if err != nil {
			log.Errorf("Failed to get canary success count: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		response := map[string][]Timeseries{
			"success": canarySuccessCount,
			"failed":  canaryFailedCount,
			"latency": canaryLatency,
		}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			log.Errorf("Failed to marshal response: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}

func getCanarySuccess(prometheusClient v1.API, checkType, exportedEndpoint string, timeframe time.Duration) ([]Timeseries, error) {
	metric := fmt.Sprintf("increase(canary_check_success_count{exported_endpoint=\"%s\", type=\"%s\"}[5m])", exportedEndpoint, checkType)
	return getMetric(prometheusClient, metric, timeframe)
}

func getCanaryFailed(prometheusClient v1.API, checkType, exportedEndpoint string, timeframe time.Duration) ([]Timeseries, error) {
	metric := fmt.Sprintf("increase(canary_check_failed_count{exported_endpoint=\"%s\", type=\"%s\"}[5m])", exportedEndpoint, checkType)
	return getMetric(prometheusClient, metric, timeframe)
}

func getCanaryLatency(prometheusClient v1.API, checkType, exportedEndpoint string, timeframe time.Duration) ([]Timeseries, error) {
	return getMetric(prometheusClient, "sum without(pod,instance)(rate(canary_check_duration_sum{exported_endpoint=\"https://httpstat.us/500\"}[5m]) / rate(canary_check_duration_count{exported_endpoint=\"https://httpstat.us/500\"}[5m]))", timeframe)
}

func getMetric(prometheusClient v1.API, metric string, timeframe time.Duration) ([]Timeseries, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rangeOptions := v1.Range{
		Start: time.Now().Add(-1 * timeframe),
		End:   time.Now(),
		Step:  5 * time.Minute,
	}
	result, warnings, err := prometheusClient.QueryRange(ctx, metric, rangeOptions)

	if err != nil {
		log.Errorf("Failed to query prometheus: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		log.Infof("Warnings: %v", warnings)
	}
	log.Debug("Result:\n%v\n", result)

	// ensure matrix result
	matrix, ok := result.(model.Matrix)
	if !ok {
		log.Errorf("Result is not a matrix")
		return nil, errors.Errorf("Result is not a matrix")
	}

	if len(matrix) < 1 {
		log.Errorf("Matrix result is empty")
		return []Timeseries{}, nil
	}

	results := []Timeseries{}

	for _, t := range matrix[0].Values {
		result := Timeseries{
			Time:  float64(t.Timestamp.Unix()),
			Value: t.Value.String(),
		}
		results = append(results, result)
	}

	return results, nil
}
