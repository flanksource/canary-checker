package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	vapi "github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

func triggerCheckOnServer(serverURL string, triggerData TriggerData) (*vapi.Response, error) { //nolint: deadcode,unused
	url := fmt.Sprintf("%s/api/triggerCheck", serverURL)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	jsonData, err := json.Marshal(triggerData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal the data")
	}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get url %s", url)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response body for url %s", url)
	}
	apiResponse := &vapi.Response{}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal json body for url %s", url)
	}
	return apiResponse, nil
}

type TriggerData struct {
	CheckKey    string `json:"checkKey"`
	CheckServer string `json:"server"`
}

func TriggerCheckHandler(w http.ResponseWriter, req *http.Request) {
	var td TriggerData
	err := json.NewDecoder(req.Body).Decode(&td)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// parsedServer := strings.Split(td.CheckServer, "@")
	// serverURL := parsedServer[1]
	// serverName := parsedServer[0]

	http.Error(w, "Not implemented", http.StatusNotImplemented)
}
