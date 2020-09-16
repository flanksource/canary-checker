package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
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

func triggerCheckOnServer(serverUrl string, triggerData TriggerData) (*api.Response, error) {
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
	apiResponse := &api.Response{}
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
