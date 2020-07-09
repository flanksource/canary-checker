package aggregate

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"

	nethttp "net/http"

	"github.com/flanksource/commons/logger"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"

	"github.com/pkg/errors"
)

var Servers []string

type AggregateCheck struct {
	Type        string                       `json:"type"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Statuses    map[string][]pkg.CheckStatus `json:"checkStatuses"`
}

type AggregateChecks []AggregateCheck

func (c AggregateChecks) Len() int {
	return len(c)
}
func (c AggregateChecks) Less(i, j int) bool {
	if c[i].Type == c[j].Type {
		return c[i].Name < c[j].Name
	}
	return c[i].Type < c[j].Type
}
func (c AggregateChecks) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

type Response struct {
	Checks  []AggregateCheck `json:"checks"`
	Servers []string         `json:"servers"`
}

func getChecksFromServer(server string) (*api.Response, error) {
	url := fmt.Sprintf("%s/api", server)
	tr := &nethttp.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &nethttp.Client{Transport: tr}
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get url %s", url)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response body for url %s", url)
	}
	apiResponse := &api.Response{}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal json body for url %s", url)
	}
	return apiResponse, nil
}

func Handler(w nethttp.ResponseWriter, req *nethttp.Request) {
	aggregateData := map[string]*AggregateCheck{}
	data := cache.GetChecks()
	for _, c := range data {
		id := c.ToString()
		aggregateData[id] = &AggregateCheck{
			Name:        c.Name,
			Type:        c.Type,
			Description: c.Description,
			Statuses: map[string][]pkg.CheckStatus{
				api.ServerName: c.Statuses,
			},
		}
	}

	servers := []string{}

	for _, serverURL := range Servers {
		apiResponse, err := getChecksFromServer(serverURL)
		if err != nil {
			logger.Errorf("Failed to get checks from server %s: %v", serverURL, err)
			continue
		}

		servers = append(servers, apiResponse.ServerName)

		for _, c := range apiResponse.Checks {
			id := c.ToString()
			ac, found := aggregateData[id]
			if found {
				ac.Statuses[apiResponse.ServerName] = c.Statuses
			} else {
				aggregateData[id] = &AggregateCheck{
					Name: c.Name,
					Type: c.Type,
					Statuses: map[string][]pkg.CheckStatus{
						apiResponse.ServerName: c.Statuses,
					},
				}
			}
		}
	}

	sort.Strings(servers)
	allServers := []string{api.ServerName}
	allServers = append(allServers, servers...)

	aggregateList := AggregateChecks{}
	for _, v := range aggregateData {
		aggregateList = append(aggregateList, *v)
	}
	sort.Sort(aggregateList)
	aggregateResponse := &Response{
		Checks:  aggregateList,
		Servers: allServers,
	}

	jsonData, err := json.Marshal(aggregateResponse)
	if err != nil {
		logger.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
		return
	}
	fmt.Fprintf(w, string(jsonData))
}
