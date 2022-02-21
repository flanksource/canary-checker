//go:build !fast

package checks

import (
	"bytes"

	"crypto/tls"
	"io/ioutil"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/utils"
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
func (c *S3Checker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.S3 {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *S3Checker) Type() string {
	return "s3"
}

func (c *S3Checker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.S3Check)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	bucket := check.Bucket
	cfg := aws.NewConfig().
		WithRegion(bucket.Region).
		WithEndpoint(bucket.Endpoint).
		WithCredentials(
			credentials.NewStaticCredentials(check.AccessKey, check.SecretKey, ""),
		)
	if check.SkipTLSVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		cfg = cfg.WithHTTPClient(&http.Client{Transport: tr})
	}
	ssn, err := session.NewSession(cfg)
	if err != nil {
		return results.Failf("Failed to create S3 session for %s: %v", bucket.Name, err)
	}
	client := s3.New(ssn)
	yes := true
	client.Config.S3ForcePathStyle = &yes

	listTimer := NewTimer()
	_, err = client.ListObjects(&s3.ListObjectsInput{Bucket: &bucket.Name})
	if err != nil {
		return results.Failf("Failed to list objects in bucket %s: %v", bucket.Name, err)
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
		return results.Failf("Failed to put object %s in bucket %s: %v", check.ObjectPath, bucket.Name, err)
	}
	updateHistogram.WithLabelValues(bucket.Endpoint, bucket.Name).Observe(updateTimer.Elapsed())

	obj, err := client.GetObject(&s3.GetObjectInput{
		Bucket: &bucket.Name,
		Key:    &check.ObjectPath,
	})

	if err != nil {
		return results.Failf("Failed to get object %s in bucket %s: %v", check.ObjectPath, bucket.Name, err)
	}
	returnedData, _ := ioutil.ReadAll(obj.Body)
	if string(returnedData) != data {
		return results.Failf("Get object doesn't match %s != %s", data, string(returnedData))
	}
	return results
}
