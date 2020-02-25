package checks

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/commons/utils"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	listHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_s3_list",
			Help:    "The total number of S3 list operations",
			Buckets: []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"endpoint", "bucket"},
	)
	updateHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_s3_update",
			Help:    "The total number of S3 update operations",
			Buckets: []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"endpoint", "bucket"},
	)
)

func init() {
	prometheus.MustRegister(listHistogram, updateHistogram)
}

type S3Checker struct{}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *S3Checker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.S3 {
		for _, result := range c.Check(conf.S3Check) {
			checks = append(checks, result)

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

		if _, err := DNSLookup(bucket.Endpoint); err != nil {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Failed to resolve DNS for %s", bucket.Endpoint),
				Endpoint: bucket.Endpoint,
			})
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
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Failed to create S3 session for %s: %v", bucket.Name, err),
				Endpoint: bucket.Name,
			})
			continue
		}
		client := s3.New(ssn)
		yes := true
		client.Config.S3ForcePathStyle = &yes

		listTimer := NewTimer()
		_, err = client.ListObjects(&s3.ListObjectsInput{Bucket: &bucket.Name})
		if err != nil {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Failed to list objects in bucket %s: %v", bucket.Name, err),
				Endpoint: bucket.Name,
			})
			continue
		}
		listHistogram.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(listTimer.Elapsed())

		data := utils.RandomString(16)
		updateTimer := NewTimer()
		_, err = client.PutObject(&s3.PutObjectInput{
			Bucket: &bucket.Name,
			Key:    &check.ObjectPath,
			Body:   bytes.NewReader([]byte(data)),
		})
		if err != nil {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Failed to put object %s in bucket %s: %v", check.ObjectPath, bucket.Name, err),
				Endpoint: bucket.Name,
			})
			continue
		}
		updateHistogram.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(updateTimer.Elapsed())

		timer := NewTimer()
		obj, err := client.GetObject(&s3.GetObjectInput{
			Bucket: &bucket.Name,
			Key:    &check.ObjectPath,
		})

		if err != nil {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Failed to get object %s in bucket %s: %v", check.ObjectPath, bucket.Name, err),
				Endpoint: bucket.Name,
			})
			continue
		}
		returnedData, _ := ioutil.ReadAll(obj.Body)
		if string(returnedData) != data {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Get object doesn't match %s != %s", data, string(returnedData)),
				Endpoint: bucket.Name,
			})
			continue
		}

		checkResult := &pkg.CheckResult{
			Pass:     true,
			Invalid:  false,
			Duration: int64(timer.Elapsed()),
			Endpoint: bucket.Endpoint,
		}
		result = append(result, checkResult)
	}
	return result
}
