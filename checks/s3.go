package checks

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/jasonlvhit/gocron"
	"github.com/jinzhu/copier"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"time"
)

var (
	s3Failed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_s3_failed",
		Help: "The total number of S3 checks failed",
	})

	s3DnsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_s3_dns_failed",
		Help: "The total number of S3 endpoint lookup failed",
	})

	lookupTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_s3_lookup_time",
			Help:    "S3 lookup in milliseconds",
			Buckets: []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"endpoint", "bucket"},
	)

	listHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "canary_check_s3_list",
			Help:        "The total number of S3 list operations",
			Buckets:     []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"endpoint", "bucket"},
	)
	readHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "canary_check_s3_read",
			Help:        "The total number of S3 read operations",
			Buckets:     []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"endpoint", "bucket"},
	)
	updateHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "canary_check_s3_update",
			Help:        "The total number of S3 update operations",
			Buckets:     []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"endpoint", "bucket"},
	)
)

func init() {
	prometheus.MustRegister(s3Failed, s3DnsFailed, lookupTime, listHistogram, readHistogram, updateHistogram)
}

type S3Checker struct{}

// Schedule: Add every check as a cron job, calls MetricProcessor with the set of metrics
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

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
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
	var result []*pkg.CheckResult
	for _, bucket := range check.Buckets {
		lookupStart := time.Now()
		_, err := DNSLookup(bucket.Endpoint)
		lookupElapsed := time.Since(lookupStart)
		if err != nil {
			result = append(result,
				c.handleError(fmt.Sprintf("Failed to resolve DNS for %s", bucket.Endpoint),
					bucket.Endpoint,
					[]pkg.Metric{}),
			)
			continue
		}

		cfg := aws.NewConfig().
			WithRegion(bucket.Region).
			WithEndpoint(bucket.Endpoint).
			WithCredentials(
				credentials.NewStaticCredentials(check.AccessKey, check.SecretKey, ""),
			)
		ssn, err := session.NewSession(cfg)
		if err != nil {
			result = append(result,
				c.handleError(fmt.Sprintf("Failed to create S3 session for %s", bucket.Name),
					bucket.Endpoint,
					[]pkg.Metric{}),
			)
			continue
		}
		client := s3.New(ssn)

		listStart := time.Now()
		_, err = client.ListObjects(&s3.ListObjectsInput{Bucket: &bucket.Name})
		if err != nil {
			result = append(result,
				c.handleError(fmt.Sprintf("Failed to list objects in bucket %s", bucket.Name),
					bucket.Endpoint,
					[]pkg.Metric{}),
			)
			continue
		}
		listHistogram.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(float64(time.Since(listStart).Milliseconds()))

		getStart := time.Now()
		obj, err := client.GetObject(&s3.GetObjectInput{
			Bucket: &bucket.Name,
			Key:    &check.ObjectPath,
		})
		if err != nil {
			result = append(result,
				c.handleError(fmt.Sprintf("Failed to get object %s in bucket %s", check.ObjectPath, bucket.Name),
					bucket.Endpoint,
					[]pkg.Metric{}),
			)
			continue
		}
		readHistogram.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(float64(time.Since(getStart).Milliseconds()))

		data, _ := ioutil.ReadAll(obj.Body)
		updateStart := time.Now()
		_, err = client.PutObject(&s3.PutObjectInput{
			Bucket: &bucket.Name,
			Key:    &check.ObjectPath,
			Body:   bytes.NewReader(data),
		})
		if err != nil {
			result = append(result,
				c.handleError(fmt.Sprintf("Failed to put object %s in bucket %s", check.ObjectPath, bucket.Name),
					bucket.Endpoint,
					[]pkg.Metric{}),
			)
			continue
		}
		updateHistogram.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(float64(time.Since(updateStart).Milliseconds()))

		m := []pkg.Metric{
			{
				Name:   "lookupTime",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"bucket": bucket.Name},
				Value:  float64(lookupElapsed.Milliseconds()),
			},
		}

		lookupTime.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(float64(lookupElapsed.Milliseconds()))
		checkResult := &pkg.CheckResult{
			Pass:     true,
			Invalid:  false,
			Duration: time.Since(getStart).Milliseconds(),
			Endpoint: bucket.Endpoint,
			Metrics:  m,
		}
		result = append(result, checkResult)
	}
	return result
}

func (c *S3Checker) handleError(errMessage string, endpoint string, metrics []pkg.Metric) *pkg.CheckResult {
	log.Print(errMessage)
	s3Failed.Inc()
	return &pkg.CheckResult{
		Pass:     false,
		Invalid:  true,
		Endpoint: endpoint,
		Metrics:  metrics,
	}
}
