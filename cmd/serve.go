package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/statuspage"
	"github.com/go-co-op/gocron"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var Serve = &cobra.Command{
	Use:   "serve",
	Short: "Start a server to execute checks ",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		config := pkg.ParseConfig(configfile)
		httpPort, _ := cmd.Flags().GetInt("httpPort")
		interval, _ := cmd.Flags().GetUint64("interval")
		dev, _ := cmd.Flags().GetBool("dev")

		var checks = []checks.Checker{
			&checks.HelmChecker{},
			&checks.DNSChecker{},
			&checks.HttpChecker{},
			&checks.IcmpChecker{},
			&checks.S3Checker{},
			&checks.S3BucketChecker{},
			&checks.DockerPullChecker{},
			&checks.DockerPushChecker{},
			&checks.PostgresChecker{},
			&checks.LdapChecker{},
			checks.NewPodChecker(),
			checks.NewNamespaceChecker(),
		}

		config.Interval = time.Duration(interval) * time.Second
		log.Infof("Running checks every %s", config.Interval)

		scheduler := gocron.NewScheduler(time.UTC)

		for _, _c := range checks {
			c := _c
			var results = make(chan *pkg.CheckResult)
			scheduler.Every(interval).Seconds().StartImmediately().Do(func() {
				go func() {
					c.Run(config, results)
				}()
			})
			go func() {
				for result := range results {
					state.AddCheck(result)
					processMetrics(c.Type(), result)
				}
			}()
		}

		scheduler.StartAsync()

		nethttp.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
		if dev {
			nethttp.HandleFunc("/", devRootPageHandler)
		} else {
			nethttp.Handle("/", nethttp.FileServer(statuspage.FS(false)))
		}
		nethttp.HandleFunc("/api", apiHandler)
		nethttp.HandleFunc("/api/aggregate", apiAggregateHandler)

		addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
		log.Infof("Starting health dashboard at http://%s", addr)
		log.Infof("Metrics dashboard can be accessed at http://%s/metrics", addr)

		if err := nethttp.ListenAndServe(addr, nil); err != nil {
			log.Fatal(errors.Wrap(err, "failed to start server"))
		}
	},
}

var counters map[string]prometheus.Counter

func processMetrics(checkType string, result *pkg.CheckResult) {
	description := result.Check.GetDescription()
	endpoint := result.Check.GetEndpoint()
	if log.IsLevelEnabled(log.InfoLevel) {
		fmt.Println(result)
	}
	pkg.OpsCount.WithLabelValues(checkType, endpoint, description).Inc()
	if result.Pass {
		pkg.Guage.WithLabelValues(checkType, description).Set(0)
		pkg.OpsSuccessCount.WithLabelValues(checkType, endpoint, description).Inc()
		if result.Duration > 0 {
			pkg.RequestLatency.WithLabelValues(checkType, endpoint, description).Observe(float64(result.Duration))
		}

		for _, m := range result.Metrics {
			switch m.Type {
			case pkg.CounterType:
				pkg.GenericCounter.WithLabelValues(checkType, description, m.Name, strconv.Itoa(int(m.Value))).Inc()
			case pkg.GaugeType:
				pkg.GenericGauge.WithLabelValues(checkType, description, m.Name).Set(m.Value)
			case pkg.HistogramType:
				pkg.GenericHistogram.WithLabelValues(checkType, description, m.Name).Observe(m.Value)
			}
		}
	} else {
		pkg.Guage.WithLabelValues(checkType, description).Set(1)
		pkg.OpsFailedCount.WithLabelValues(checkType, endpoint, description).Inc()
	}
}

type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format("2006-01-02 15:04:05"))
	return []byte(stamp), nil
}

func (t *JSONTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		*t = JSONTime(time.Time{})
		return nil
	}
	x, err := time.Parse("2006-01-02 15:04:05", s)
	*t = JSONTime(x)
	return err
}

type CheckStatus struct {
	Status   bool     `json:"status"`
	Invalid  bool     `json:"invalid"`
	Time     JSONTime `json:"time"`
	Duration int      `json:"duration"`
	Message  string   `json:"message"`
}

type Check struct {
	Type string `json:"type"`
	Name string `json:"name"`

	Statuses []CheckStatus `json:"checkStatuses"`
}

type Checks []Check

func (c Checks) Len() int {
	return len(c)
}
func (c Checks) Less(i, j int) bool {
	if c[i].Type == c[j].Type {
		return c[i].Name < c[j].Name
	}
	return c[i].Type < c[j].Type
}
func (c Checks) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c Check) ToString() string {
	return fmt.Sprintf("%s;%s", c.Type, c.Name)
}

type AggregateCheck struct {
	Type string `json:"type"`
	Name string `json:"name"`

	Statuses map[string][]CheckStatus `json:"checkStatuses"`
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

type AggregateResponse struct {
	Checks  []AggregateCheck `json:"checks"`
	Servers []string         `json:"servers"`
}

type APIResponse struct {
	ServerName string  `json:"serverName"`
	Checks     []Check `json:"checks"`
}

type State struct {
	Checks map[string]Check
	mtx    sync.Mutex
}

func (s *State) AddCheck(result *pkg.CheckResult) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	check := Check{
		Type: result.Check.GetType(),
		Name: result.Check.GetEndpoint(),
		Statuses: []CheckStatus{
			{
				Status:   result.Pass,
				Invalid:  result.Invalid,
				Duration: int(result.Duration),
				Time:     JSONTime(time.Now().UTC()),
				Message:  result.Message,
			},
		},
	}

	key := fmt.Sprintf("%s/%s", check.Type, check.Name)
	log.Debugf("Set key %s to state", key)

	lastCheck, found := s.Checks[key]
	if found {
		check.Statuses = append(check.Statuses, lastCheck.Statuses...)
		if len(check.Statuses) > maxStatusCheckCount {
			check.Statuses = check.Statuses[:maxStatusCheckCount]
		}
	}
	s.Checks[key] = check
}

func (s *State) GetChecks() []Check {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	result := Checks{}

	for _, m := range s.Checks {
		result = append(result, m)
	}

	sort.Sort(&result)

	return result
}

var maxStatusCheckCount = 5
var state = &State{Checks: map[string]Check{}}
var aggregateServers []string
var serverName string

func getChecksFromServer(server string) (*APIResponse, error) {
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
	apiResponse := &APIResponse{}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal json body for url %s", url)
	}
	return apiResponse, nil
}

func apiHandler(w nethttp.ResponseWriter, req *nethttp.Request) {
	apiResponse := &APIResponse{
		ServerName: serverName,
		Checks:     state.GetChecks(),
	}
	jsonData, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
		return
	}
	fmt.Fprintf(w, string(jsonData))
}

func apiAggregateHandler(w nethttp.ResponseWriter, req *nethttp.Request) {
	aggregateData := map[string]*AggregateCheck{}
	data := state.GetChecks()
	for _, c := range data {
		id := c.ToString()
		aggregateData[id] = &AggregateCheck{
			Name: c.Name,
			Type: c.Type,
			Statuses: map[string][]CheckStatus{
				serverName: c.Statuses,
			},
		}
	}

	servers := []string{}

	for _, serverURL := range aggregateServers {
		apiResponse, err := getChecksFromServer(serverURL)
		if err != nil {
			log.Errorf("Failed to get checks from server %s: %v", serverURL, err)
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
					Statuses: map[string][]CheckStatus{
						apiResponse.ServerName: c.Statuses,
					},
				}
			}
		}
	}

	sort.Strings(servers)
	allServers := []string{serverName}
	allServers = append(allServers, servers...)

	aggregateList := AggregateChecks{}
	for _, v := range aggregateData {
		aggregateList = append(aggregateList, *v)
	}
	sort.Sort(aggregateList)
	aggregateResponse := &AggregateResponse{
		Checks:  aggregateList,
		Servers: allServers,
	}

	jsonData, err := json.Marshal(aggregateResponse)
	if err != nil {
		log.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
		return
	}
	fmt.Fprintf(w, string(jsonData))
}

func devRootPageHandler(w nethttp.ResponseWriter, req *nethttp.Request) {
	if req.URL.Path != "/" {
		w.WriteHeader(nethttp.StatusNotFound)
		fmt.Fprintf(w, "{\"error\": \"page not found\", \"checks\": []}")
		return
	}
	body, err := ioutil.ReadFile("statuspage/index.html")
	if err != nil {
		log.Errorf("Failed to read html file: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
	}
	fmt.Fprintf(w, string(body))
}

func init() {
	Serve.Flags().Int("httpPort", 8080, "Port to expose a health dashboard ")
	Serve.Flags().Uint64("interval", 30, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
	Serve.Flags().Bool("dev", false, "Run in development mode")
	Serve.Flags().IntVar(&maxStatusCheckCount, "maxStatusCheckCount", 5, "Maximum number of past checks in the status page")
	Serve.Flags().StringSliceVar(&aggregateServers, "aggregateServers", []string{}, "Aggregate check results from multiple servers in the status page")
	Serve.Flags().StringVar(&serverName, "name", "local", "Server name shown in aggregate dashboard")
}
