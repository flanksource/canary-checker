package checks

import (
	"fmt"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/jasonlvhit/gocron"
	"github.com/jinzhu/copier"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	s3Failed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_s3_failed",
		Help: "The total number of S3 checks failed",
	})

	lookupTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_s3_lookup_time",
			Help:    "S3 lookup in milliseconds",
			Buckets: []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"s3", "lookup"},
	)

	listCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_s3_list",
		Help: "The total number of S3 list operations",
	})
	updateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_s3_update",
		Help: "The total number of S3 update operations",
	})
	readCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_s3_read",
		Help: "The total number of S3 read operations",
	})
)

func init() {
	prometheus.MustRegister(s3Failed, lookupTime, listCount, updateCount, readCount)
}

type S3Checker struct {}

func (c *S3Checker) Schedule(config pkg.Config, interval uint64, mp MetricProcessor) {
	for _, conf := range config.S3 {
		s3Check := pkg.S3Check{}
		if err := copier.Copy(&s3Check, &conf.S3Check); err != nil {
			log.Printf("error copying %v", err)
		}
		gocron.Every(interval).Seconds().Do(func() {
			metrics := c.Check(s3Check)
			mp(metrics)
		})
	}
}

func (c *S3Checker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.S3 {
		for _, result := range c.Check(conf.S3Check) {
			checks = append(checks, result)
			fmt.Println(result)
		}
	}
	return checks
}

// Type: returns checker type
func (c *S3Checker) Type() string {
	return "s3"
}

func (c *S3Checker) Check(check pkg.S3Check) []*pkg.CheckResult {
	return nil
}

