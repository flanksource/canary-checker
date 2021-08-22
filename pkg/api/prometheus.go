package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

type PrometheusGraphData struct {
	CheckType  string `json:"checkType"`
	CheckKey   string `json:"checkKey"`
	CanaryName string `json:"canaryName"`
	Timeframe  int    `json:"timeframe"`
}

type Timeseries struct {
	Time  float64 `json:"time"`
	Value string  `json:"value"`
}

func PrometheusGraphHandler(w http.ResponseWriter, req *http.Request) {
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

	if runner.Prometheus == nil {
		http.Error(w, "Prometheus not configured", http.StatusInternalServerError)
		return
	}

	canarySuccessCount, err := getCanarySuccess(runner.Prometheus, pg.CheckType, pg.CheckKey, pg.CanaryName, timeframe)
	if err != nil {
		log.Errorf("Failed to get canary success count: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	canaryFailedCount, err := getCanaryFailed(runner.Prometheus, pg.CheckType, pg.CheckKey, pg.CanaryName, timeframe)
	if err != nil {
		log.Errorf("Failed to get canary success count: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	canaryLatency, err := getCanaryLatency(runner.Prometheus, pg.CheckType, pg.CheckKey, pg.CanaryName, timeframe)
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
	_, err = w.Write(jsonResponse)
	if err != nil {
		log.Errorf("Failed to write response: %v", err)
		return
	}
}

func getCanarySuccess(prometheusClient v1.API, checkType, exportedEndpoint, checkName string, timeframe time.Duration) ([]Timeseries, error) {
	metric := fmt.Sprintf("increase(canary_check_success_count{exported_endpoint=\"%s\", type=\"%s\", name=\"%s\"}[5m])", exportedEndpoint, checkType, checkName)
	return getMetric(prometheusClient, metric, timeframe)
}

func getCanaryFailed(prometheusClient v1.API, checkType, exportedEndpoint, checkName string, timeframe time.Duration) ([]Timeseries, error) {
	metric := fmt.Sprintf("increase(canary_check_failed_count{exported_endpoint=\"%s\", type=\"%s\", name=\"%s\"}[5m])", exportedEndpoint, checkType, checkName)
	return getMetric(prometheusClient, metric, timeframe)
}

func getCanaryLatency(prometheusClient v1.API, checkType, exportedEndpoint, checkName string, timeframe time.Duration) ([]Timeseries, error) {
	metric := fmt.Sprintf("sum without(pod,instance)(rate(canary_check_duration_sum{exported_endpoint=\"%s\", type=\"%s\", name=\"%s\"}[5m]) / rate(canary_check_duration_count{exported_endpoint=\"%s\", type=\"%s\", name=\"%s\"}[5m]))", exportedEndpoint, checkType, checkName, exportedEndpoint, checkType, checkName)
	return getMetric(prometheusClient, metric, timeframe)
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
	log.Infof("Query: %s", metric)
	log.Debugf("Result:\n%v\n", result)

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
