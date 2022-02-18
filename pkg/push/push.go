package push

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"

	"github.com/flanksource/commons/logger"
)

func Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		logger.Errorf("%v method on /api/push endpoint is not allowed", req.Method)
		fmt.Fprintf(w, "%v method not allowed", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.Body == nil {
		logger.Debugf("missing request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()
	data := QueueData{
		Check:  pkg.Check{},
		Status: pkg.CheckStatus{},
	}
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Errorf("error reading the request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(reqBody, &data); err != nil {
		logger.Errorf("failed to unmarshal json body. Error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cache.PostgresCache.Add(data.Check, data.Status)
	w.WriteHeader(http.StatusCreated)
}

func PostDataToServer(server string, body io.Reader) (err error) {
	url := fmt.Sprintf("%s/api/push", server)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Post(url, "application/json", body)
	if err != nil {
		return err
	}
	defer func() { err = resp.Body.Close() }()
	return err
}
