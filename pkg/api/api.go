package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
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
	fmt.Fprintf(w, string(jsonData))
}
