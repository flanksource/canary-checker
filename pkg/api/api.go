package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	apiResponse := &Response{
		RunnerName: runner.RunnerName,
		Checks:     cache.GetChecks(),
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
