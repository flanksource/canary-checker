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
	cmap "github.com/orcaman/concurrent-map"
)

var Servers []string
var PivotByNamespace bool
var checkCache = cmap.New()

type AggregateCheck struct { // nolint: golint
	CanaryName   string                       `json:"canaryName"`
	Description  string                       `json:"description"`
	DisplayType  string                       `json:"displayType"`
	Endpoint     string                       `json:"endpoint"`
	Health       map[string]CheckHealth       `json:"health"`
	IconURL      string                       `json:"iconURL"`
	Interval     uint64                       `json:"interval"`
	Key          string                       `json:"key"`
	Labels       map[string]string            `json:"labels"`
	Name         string                       `json:"name"`
	Namespace    string                       `json:"namespace"`
	Owner        string                       `json:"owner"`
	RunnerLabels map[string]string            `json:"runner"`
	Schedule     string                       `json:"schedule"`
	ServerURL    string                       `json:"serverURL"`
	Severity     string                       `json:"severity"`
	Statuses     map[string][]pkg.CheckStatus `json:"checkStatuses"`
	Type         string                       `json:"type"`
}

type CheckHealth struct {
	Latency Latency `json:"latency"`
	Uptime  Uptime  `json:"uptime"`
}

type Latency struct {
	Percentile99 string `json:"p99,omitempty"`
	Percentile97 string `json:"p97,omitempty"`
	Percentile95 string `json:"p95,omitempty"`
	Rolling1H    string `json:"rolling1h"`
}

type Uptime struct {
	Rolling1H string `json:"rolling1h"`
	Uptime    string `json:"uptime,omitempty"`
}

type AggregateChecks []AggregateCheck // nolint: golint

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
	queryParams := req.URL.Query()
	latencyTime := queryParams.Get("latency")
	uptime := queryParams.Get("uptime")
	servers := []string{}
	allServers := []string{}

	if PivotByNamespace {
		namespaceMap := map[string]bool{}
		for _, c := range data {
			namespaceMap[c.GetNamespace()] = true
		}
		for n := range namespaceMap {
			servers = append(servers, n)
		}

		for _, c := range data {
			ac, found := aggregateData[c.ID()]
			if found {
				ac.Health[c.GetNamespace()] = CheckHealth{getLatenciesFromPrometheus(c.Key, latencyTime, c.Latency), getUptimeFromPrometheus(c.Key, uptime, c.Uptime)}
				ac.Statuses[c.GetNamespace()] = c.Statuses
			} else {
				aggregateData[c.ID()] = &AggregateCheck{
					Key:          c.Key,
					Name:         c.GetName(),
					Namespace:    "",
					Labels:       c.CheckCanary.Labels,
					RunnerLabels: pkg.RunnerLabels,
					CanaryName:   c.CanaryName,
					Type:         c.Type,
					Description:  c.Description,
					Endpoint:     c.Endpoint,
					Interval:     c.Interval,
					Schedule:     c.Schedule,
					Owner:        c.Owner,
					Severity:     c.Severity,
					ServerURL:    c.GetNamespace(),
					IconURL:      c.Icon,
					DisplayType:  c.DisplayType,
					Health: map[string]CheckHealth{
						c.GetNamespace(): {getLatenciesFromPrometheus(c.Key, latencyTime, c.Latency), getUptimeFromPrometheus(c.Key, uptime, c.Uptime)},
					},
					Statuses: map[string][]pkg.CheckStatus{
						c.GetNamespace(): c.Statuses,
					},
				}
			}
		}
	} else {
		localServerID := api.RunnerName
		for _, c := range data {
			aggregateData[c.ID()] = &AggregateCheck{
				Key:          c.Key,
				Name:         c.GetName(),
				Namespace:    c.GetNamespace(),
				Labels:       c.CheckCanary.Labels,
				RunnerLabels: pkg.RunnerLabels,
				CanaryName:   c.CanaryName,
				Type:         c.Type,
				Description:  c.Description,
				Endpoint:     c.Endpoint,
				Interval:     c.Interval,
				Schedule:     c.Schedule,
				Owner:        c.Owner,
				Severity:     c.Severity,
				IconURL:      c.Icon,
				DisplayType:  c.DisplayType,
				ServerURL:    "local",
				Health: map[string]CheckHealth{
					localServerID: {getLatenciesFromPrometheus(c.Key, latencyTime, c.Latency), getUptimeFromPrometheus(c.Key, uptime, c.Uptime)},
				},
				Statuses: map[string][]pkg.CheckStatus{
					localServerID: c.Statuses,
				},
			}
		}

		for _, serverURL := range Servers {
			apiResponse := getChecksFromServer(serverURL)
			serverID := fmt.Sprintf("%s@%s", apiResponse.RunnerName, serverURL)
			servers = append(servers, serverID)

			for _, c := range apiResponse.Checks {
				ac, found := aggregateData[c.ID()]
				if found {
					ac.Health[serverID] = CheckHealth{getLatenciesFromPrometheus(c.Key, latencyTime, c.Latency), getUptimeFromPrometheus(c.Key, uptime, c.Uptime)}
					ac.Statuses[serverID] = c.Statuses
				} else {
					aggregateData[c.ID()] = &AggregateCheck{
						Key:          c.Key,
						Name:         c.GetName(),
						Namespace:    c.GetNamespace(),
						Labels:       c.CheckCanary.Labels,
						RunnerLabels: pkg.RunnerLabels,
						CanaryName:   c.CanaryName,
						Type:         c.Type,
						Description:  c.Description,
						Endpoint:     c.Endpoint,
						Interval:     c.Interval,
						Schedule:     c.Schedule,
						Owner:        c.Owner,
						Severity:     c.Severity,
						IconURL:      c.Icon,
						DisplayType:  c.DisplayType,
						ServerURL:    serverURL,
						Health: map[string]CheckHealth{
							serverID: {getLatenciesFromPrometheus(c.Key, latencyTime, c.Latency), getUptimeFromPrometheus(c.Key, uptime, c.Uptime)},
						},
						Statuses: map[string][]pkg.CheckStatus{
							serverID: c.Statuses,
						},
					}
				}
			}
		}

		allServers = []string{localServerID}
	}

	sort.Strings(servers)
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
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	_, err = w.Write(jsonData)
	if err != nil {
		logger.Errorf("Failed to write data: %v", err)
		return
	}
}

func getLatenciesFromPrometheus(checkKey string, duration string, rolling1hLatency string) (latency Latency) {
	latency.Rolling1H = rolling1hLatency
	if api.Prometheus != nil && duration != "" {
		value, err := api.Prometheus.GetHistogramQuantileLatency("0.95", checkKey, duration)
		if err != nil {
			logger.Debugf("failed to execute query: %v", err)
			return
		}
		latency.Percentile95 = value
		value, err = api.Prometheus.GetHistogramQuantileLatency("0.97", checkKey, duration)
		if err != nil {
			logger.Debugf("failed to execute query: %v", err)
			return
		}
		latency.Percentile97 = value
		value, err = api.Prometheus.GetHistogramQuantileLatency("0.99", checkKey, duration)
		if err != nil {
			logger.Debugf("failed to execute query: %v", err)
			return
		}
		latency.Percentile99 = value
	}
	return
}

func getUptimeFromPrometheus(checkKey, duration, rolling1huptime string) (uptime Uptime) {
	uptime.Rolling1H = rolling1huptime
	if api.Prometheus != nil && duration != "" {
		value, err := api.Prometheus.GetUptime(checkKey, duration)
		if err != nil {
			logger.Debugf("failed to execute query: %v", err)
			return
		}
		uptime.Uptime = value
	}
	return
}
