package spec

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
)

func CanaryHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		logger.Errorf("%v method on /api/canary endpoint is not allowed", req.Method)
		fmt.Fprintf(w, "%v method not allowed", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	queryParams := req.URL.Query()
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")
	runner := queryParams.Get("runner")

	if name == "" || namespace == "" || runner == "" {
		logger.Errorf("parameter key is required")
		fmt.Fprintf(w, "parameter key is required")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	check := cache.InMemoryCache.GetCheckFromID(fmt.Sprintf("%s/%s/%s", runner, namespace, name))
	if check == nil {
		logger.Errorf("check not found")
		fmt.Fprintf(w, "check not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	jsonData, err := json.Marshal(check.Canary)
	if err != nil {
		logger.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\"}")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}
