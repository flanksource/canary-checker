package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"

	"github.com/flanksource/commons/logger"
)

type QueueData struct {
	Check  pkg.Check       `json:",inline"`
	Status pkg.CheckStatus `json:",inline"`
}

func PushHandler(w http.ResponseWriter, req *http.Request) {
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
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(reqBody, &data); err != nil {
		errorResonse(w, err, http.StatusBadRequest)
		return
	}
	if data.Check.ID != "" && data.Check.CanaryID == "" {
		check, err := db.GetCheck(data.Check.ID)
		if check == nil && err == nil {
			errorResonse(w, fmt.Errorf("check not found: %s ", data.Check.ID), http.StatusNotFound)
			return
		} else if err != nil {
			errorResonse(w, fmt.Errorf("failed to lookup check: %s ", err), http.StatusInternalServerError)
			return
		}
		data.Check.CanaryID = check.CanaryID
	} else if data.Check.ID == "" {
		canary, err := db.FindCanary(data.Check.Namespace, data.Check.Name)
		if err != nil {
			errorResonse(w, fmt.Errorf("failed to lookup canary: %s ", err), http.StatusInternalServerError)
			return
		}
		if canary == nil {
			canary = &pkg.Canary{
				Name:      data.Check.Name,
				Namespace: data.Check.Namespace,
			}
			if err := db.CreateCanary(canary); err != nil {
				errorResonse(w, fmt.Errorf("failed to create canary: %s ", err), http.StatusInternalServerError)
				return
			}
			data.Check.CanaryID = canary.ID.String()
			if err := db.CreateCheck(*canary, &data.Check); err != nil {
				errorResonse(w, fmt.Errorf("failed to create canary: %s ", err), http.StatusInternalServerError)
				return
			}
		}
		data.Check.CanaryID = canary.ID.String()
	}
	cache.PostgresCache.Add(data.Check, data.Status)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("pushed to %s", data.Check.ID)))
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
	responseBody, _ := ioutil.ReadAll(resp.Body)
	logger.Tracef("[%s] %d %s", server, resp.StatusCode, responseBody)
	if resp.StatusCode != 201 {
		return errors.New(string(responseBody))
	}
	return err
}
