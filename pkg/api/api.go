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

type Response struct {
	RunnerName string     `json:"runnerName"`
	Checks     pkg.Checks `json:"checks"`
}

func Handler(w http.ResponseWriter, req *http.Request) {
	queryParams := req.URL.Query()
	count := queryParams.Get("count")
	var c int64
	var err error
	if count != "" {
		c, err = strconv.ParseInt(count, 10, 64)
		if err != nil {
			logger.Errorf("error converting count to int: %v", err)
			fmt.Fprintf(w, "error converting count to int")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		c = int64(cache.InMemoryCacheSize)
	}
	timeString := queryParams.Get("since")
	var timeDuration *time.Duration
	if timeString != "" {
		duration, err := time.ParseDuration(timeString)
		if err != nil {
			logger.Errorf("since value not a valid duration")
			fmt.Fprintf(w, "since value not a valid duration")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		timeDuration = &duration
	}
	checkKey := queryParams.Get("key")

	apiResponse := &Response{
		RunnerName: runner.RunnerName,
		Checks:     cache.QueryChecks(cache.CacheChain, c, timeDuration, checkKey),
	}
	jsonData, err := json.Marshal(apiResponse)
	if err != nil {
		logger.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}
