package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
)

var DefaultWindow = "1h"

type Response struct {
	Duration   int        `json:"duration,omitempty"`
	RunnerName string     `json:"runnerName"`
	Checks     pkg.Checks `json:"checks"`
}
type DetailResponse struct {
	Duration   int              `json:"duration,omitempty"`
	RunnerName string           `json:"runnerName"`
	Status     []pkg.Timeseries `json:"status"`
}

func About(w http.ResponseWriter, req *http.Request) {
	data, _ := json.Marshal(map[string]interface{}{
		"Timestamp": time.Now(),
		"Version":   runner.Version,
	})
	fmt.Fprint(w, string(data))
}

func Dump(w http.ResponseWriter, req *http.Request) {
	data, _ := json.Marshal(cache.InMemoryCache)
	fmt.Fprint(w, string(data))
}

func parseQuery(req *http.Request) (*cache.QueryParams, error) {
	queryParams := req.URL.Query()
	count := queryParams.Get("count")
	var c int64
	var err error
	if count != "" {
		c, err = strconv.ParseInt(count, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("count must be a number: %s", count)
		}
	} else {
		c = int64(cache.InMemoryCacheSize)
	}
	since := queryParams.Get("since")
	if since == "" {
		since = queryParams.Get("start")
	}
	if since == "" {
		since = DefaultWindow
	}
	until := queryParams.Get("until")
	if until == "" {
		until = queryParams.Get("end")
	}
	q := cache.QueryParams{
		Start:       since,
		End:         until,
		Check:       queryParams.Get("key"),
		StatusCount: int(c),
		Trace:       queryParams.Get("trace") == "true",
	}

	if err := q.Validate(); err != nil {
		return nil, err
	}

	return &q, nil
}

func CheckDetails(w http.ResponseWriter, req *http.Request) {
	q, err := parseQuery(req)
	if err != nil {
		errorResonse(w, err, http.StatusBadRequest)
		return
	}

	start := time.Now()
	results, err := cache.CacheChain.QueryStatus(*q)
	if err != nil {
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}
	apiResponse := &DetailResponse{
		RunnerName: runner.RunnerName,
		Status:     results,
		Duration:   int(time.Since(start).Milliseconds()),
	}
	jsonData, err := json.Marshal(apiResponse)
	if err != nil {
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}

func CheckSummary(w http.ResponseWriter, req *http.Request) {
	q, err := parseQuery(req)
	if err != nil {
		errorResonse(w, err, http.StatusBadRequest)
		return
	}

	start := time.Now()
	results, err := cache.CacheChain.Query(*q)
	if err != nil {
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}

	apiResponse := &Response{
		RunnerName: runner.RunnerName,
		Checks:     results,
		Duration:   int(time.Since(start).Milliseconds()),
	}
	jsonData, err := json.Marshal(apiResponse)
	if err != nil {
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}
