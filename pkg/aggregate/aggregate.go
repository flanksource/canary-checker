package aggregate

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"sort"

	"github.com/flanksource/commons/logger"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"

	"github.com/pkg/errors"
)

var Servers []string

type AggregateCheck struct {
	ServerURL   string                       `json:"serverURL"`
	Key         string                       `json:"key"`
	Type        string                       `json:"type"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Endpoint    string                       `json:"endpoint"`
	Health      map[string]CheckHealth       `json:"health"`
	Statuses    map[string][]pkg.CheckStatus `json:"checkStatuses"`
}

type CheckHealth struct {
	Latency string `json:"latency"`
	Uptime  string `json:"uptime"`
}

type AggregateChecks []AggregateCheck

func (c AggregateChecks) Len() int {
	return len(c)
}
func (c AggregateChecks) Less(i, j int) bool {
	if c[i].Type == c[j].Type {
		return c[i].Key < c[j].Key
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

	localServerId := fmt.Sprintf("%s@local", api.ServerName)
	var aggregateDataKey string
	for _, c := range data {
		aggregateDataKey = c.Key + c.Description
		aggregateData[aggregateDataKey] = &AggregateCheck{
			Key:         c.Key,
			Name:        c.Name,
			Type:        c.Type,
			Description: c.Description,
			Endpoint:    c.Endpoint,
			ServerURL:   "local",
			Health: map[string]CheckHealth{
				localServerId: {c.Latency, c.Uptime},
			},
			Statuses: map[string][]pkg.CheckStatus{
				localServerId: c.Statuses,
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
		serverId := fmt.Sprintf("%s@%s", apiResponse.ServerName, serverURL)
		servers = append(servers, serverId)

		for _, c := range apiResponse.Checks {
			aggregateDataKey = c.Key + c.Description
			ac, found := aggregateData[aggregateDataKey]
			if found {
				ac.Health[serverId] = CheckHealth{c.Latency, c.Uptime}
				ac.Statuses[serverId] = c.Statuses
			} else {
				aggregateData[aggregateDataKey] = &AggregateCheck{
					Key:         c.Key,
					Name:        c.Name,
					Type:        c.Type,
					Description: c.Description,
					Endpoint:    c.Endpoint,
					ServerURL:   serverURL,
					Health: map[string]CheckHealth{
						serverId: {c.Latency, c.Uptime},
					},
					Statuses: map[string][]pkg.CheckStatus{
						serverId: c.Statuses,
					},
				}
			}
		}
	}

	sort.Strings(servers)
	allServers := []string{localServerId}
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
	w.Write(jsonData)
}
