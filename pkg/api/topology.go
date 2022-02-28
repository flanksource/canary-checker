package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
)

func Topology(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		errorResonse(w, fmt.Errorf("unsupported method %s", req.Method), http.StatusMethodNotAllowed)
		return
	}

	results, err := topology.Query(topology.NewTopologyParams(req.URL.Query()))
	if err != nil {
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		errorResonse(w, errors.New("error marshalling json"), http.StatusInternalServerError)
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}
