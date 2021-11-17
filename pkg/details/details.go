package details

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
)

func Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		logger.Errorf("%v method on /api/details endpoint is not allowed", req.Method)
		fmt.Fprintf(w, "%v method not allowed", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	queryParams := req.URL.Query()
	key := queryParams.Get("key")
	time := queryParams.Get("time")
	if key == "" || time == "" {
		logger.Errorf("key and time are required parameters")
		fmt.Fprintf(w, "key and time are required parameters")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	detail := cache.CacheChain.GetDetails(key, time)
	jsonData, err := json.Marshal(detail)
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
