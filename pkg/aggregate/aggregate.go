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

	"github.com/orcaman/concurrent-map"
)

var Servers []string
var checkCache = cmap.New()

type AggregateCheck struct {
	ServerURL   string                       `json:"serverURL"`
	Key         string                       `json:"key"`
	Type        string                       `json:"type"`
	Name        string                       `json:"name"`
	Namespace   string                       `json:"namespace"`
	CanaryName  string                       `json:"canaryName"`
	Description string                       `json:"description"`
	Endpoint    string                       `json:"endpoint"`
	Health      map[string]CheckHealth       `json:"health"`
	Statuses    map[string][]pkg.CheckStatus `json:"checkStatuses"`
	Interval    uint64                       `json:"interval"`
	Owner       string                       `json:"owner"`
	Severity    string                       `json:"severity"`
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

func doChecksFromServer(server string) {
	url := fmt.Sprintf("%s/api", server)
	tr := &nethttp.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &nethttp.Client{Transport: tr}
	resp, err := client.Get(url)
	if err != nil {
		logger.Errorf("failed to get url %s. Error: %v", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("failed to read response body for url %s. Error: %v", url, err)
	}
	apiResponse := &api.Response{}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		logger.Errorf("failed to unmarshal json body for url %s. Error: %v", url, err)
	}
	checkCache.Set(server, apiResponse)
}
func getChecksFromServer(server string) *api.Response {
	go doChecksFromServer(server)
	apiResponse, exists := checkCache.Get(server)
	if !exists {
		return &api.Response{}
	}
	if response, ok := apiResponse.(*api.Response); ok {
		return response
	}
	return &api.Response{}
}

func Handler(w nethttp.ResponseWriter, req *nethttp.Request) {
	aggregateData := map[string]*AggregateCheck{}
	data := cache.GetChecks()

	localServerId := api.ServerName
	for _, c := range data {
		aggregateData[c.ID()] = &AggregateCheck{
			Key:         c.Key,
			Name:        c.GetName(),
			Namespace:   c.GetNamespace(),
			CanaryName:  c.CanaryName,
			Type:        c.Type,
			Description: c.Description,
			Endpoint:    c.Endpoint,
			Interval:    c.Interval,
			Owner:       c.Owner,
			Severity:    c.Severity,
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
		apiResponse := getChecksFromServer(serverURL)
		serverId := fmt.Sprintf("%s@%s", apiResponse.ServerName, serverURL)
		servers = append(servers, serverId)

		for _, c := range apiResponse.Checks {
			ac, found := aggregateData[c.ID()]
			if found {
				ac.Health[serverId] = CheckHealth{c.Latency, c.Uptime}
				ac.Statuses[serverId] = c.Statuses
			} else {
				aggregateData[c.ID()] = &AggregateCheck{
					Key:         c.Key,
					Name:        c.GetName(),
					Namespace:   c.GetNamespace(),
					CanaryName:  c.CanaryName,
					Type:        c.Type,
					Description: c.Description,
					Endpoint:    c.Endpoint,
					Interval:    c.Interval,
					Owner:       c.Owner,
					Severity:    c.Severity,
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
