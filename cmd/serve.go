package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/jasonlvhit/gocron"
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

		for _, _c := range checks {
			c := _c
			var results = make(chan *pkg.CheckResult)
			gocron.Every(interval).Seconds().From(gocron.NextTick()).Do(func() {
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

		gocron.Start()

		nethttp.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
		nethttp.HandleFunc("/", statusPageHandler)
		nethttp.HandleFunc("/api", apiPageHandler)

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
	description := ""
	switch result.Check.(type) {
	case pkg.Describable:
		description = result.Check.(pkg.Describable).GetDescription()
	}
	if log.IsLevelEnabled(log.InfoLevel) {
		fmt.Println(result)
	}
	pkg.OpsCount.WithLabelValues(checkType, result.Endpoint, description).Inc()
	if result.Pass {
		pkg.Guage.WithLabelValues(checkType, description).Set(0)
		pkg.OpsSuccessCount.WithLabelValues(checkType, result.Endpoint, description).Inc()
		if result.Duration > 0 {
			pkg.RequestLatency.WithLabelValues(checkType, result.Endpoint, description).Observe(float64(result.Duration))
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
		pkg.OpsFailedCount.WithLabelValues(checkType, result.Endpoint, description).Inc()
	}
}

type Check struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Status   bool   `json:"status"`
	Invalid  bool   `json:"invalid"`
	Duration int    `json:"duration"`
}

type Checks []Check

func (c Checks) Len() int {
	return len(c)
}
func (c Checks) Less(i, j int) bool {
	if c[i].Type == c[j].Name {
		return c[i].Name < c[j].Name
	}
	return c[i].Type < c[j].Type
}
func (c Checks) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

type State struct {
	Checks map[string]Check
	mtx    sync.Mutex
}

func (s *State) AddCheck(result *pkg.CheckResult) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	check := Check{}

	switch result.Check.(type) {
	case pkg.WithType:
		check.Type = result.Check.(pkg.WithType).GetType()
	default:
		log.Errorf("Check %v does not have type", result.Check)
		return
	}

	check.Name = result.Endpoint
	check.Duration = int(result.Duration)
	check.Status = result.Pass
	check.Invalid = result.Invalid

	key := fmt.Sprintf("%s/%s", check.Type, check.Name)
	log.Debugf("Set key %s to state", key)
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

var state = &State{Checks: map[string]Check{}}

func statusPageHandler(w nethttp.ResponseWriter, req *nethttp.Request) {
	if req.URL.Path != "/" {
		w.WriteHeader(nethttp.StatusNotFound)
		fmt.Fprintf(w, "{\"error\": \"page not found\", \"checks\": []}")
		return
	}
	body, err := ioutil.ReadFile("index.html")
	if err != nil {
		log.Errorf("Failed to read html file: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
	}
	fmt.Fprintf(w, string(body))
}

func apiPageHandler(w nethttp.ResponseWriter, req *nethttp.Request) {
	data := state.GetChecks()
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\", \"checks\": []}")
		return
	}
	fmt.Fprintf(w, string(jsonData))
}

func init() {
	Serve.Flags().Int("httpPort", 8080, "Port to expose a health dashboard ")
	Serve.Flags().Uint64("interval", 30, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
}
