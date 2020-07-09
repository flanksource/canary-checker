package cmd

import (
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"strconv"
	"time"

	_ "net/http/pprof"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/aggregate"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/statuspage"
	"github.com/flanksource/commons/logger"
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
						cache.AddCheck("", result)
						metrics.Record("", "", result)
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
		nethttp.HandleFunc("/api", api.Handler)
		nethttp.HandleFunc("/api/aggregate", aggregate.Handler)

		addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
		log.Infof("Starting health dashboard at http://%s", addr)
		log.Infof("Metrics dashboard can be accessed at http://%s/metrics", addr)

		if err := nethttp.ListenAndServe(addr, nil); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
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
	Serve.Flags().IntVar(&cache.Size, "maxStatusCheckCount", 5, "Maximum number of past checks in the status page")
	Serve.Flags().StringSliceVar(&aggregate.Servers, "aggregateServers", []string{}, "Aggregate check results from multiple servers in the status page")
	Serve.Flags().StringVar(&api.ServerName, "name", "local", "Server name shown in aggregate dashboard")
}
