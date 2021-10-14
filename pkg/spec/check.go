package spec

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
)

func CheckHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		logger.Errorf("%v method on /api/spec endpoint is not allowed", req.Method)
		fmt.Fprintf(w, "%v method not allowed", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	queryParams := req.URL.Query()
	key := queryParams.Get("key")
	if key == "" {
		logger.Errorf("parameter key is required")
		fmt.Fprintf(w, "parameter key is required")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	check := cache.Cache.GetCheckFromKey(key)
	if check == nil {
		logger.Errorf("no check found for key %v", key)
		fmt.Fprintf(w, "no check found for key %v", key)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	jsonData, err := json.Marshal(check)
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
