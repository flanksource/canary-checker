//go:build !fast

package checks

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/connection"
	"github.com/henvic/httpretty"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/flanksource/canary-checker/api/v1"
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

	if err := check.AWSConnection.Populate(ctx); err != nil {
		return results.Failf("failed to populate aws connection: %v", err)
	}

	cfg, err := GetAWSConfig(ctx, check.AWSConnection)
	if err != nil {
		return results.Failf("Failed to get AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = check.S3Connection.UsePathStyle
	})

	listTimer := NewTimer()
	_, err = client.ListObjects(ctx, &s3.ListObjectsInput{Bucket: &check.BucketName})
	if err != nil {
		return results.Failf("Failed to list objects in bucket %s: %v", check.BucketName, err)
	}
	listHistogram.WithLabelValues(check.AWSConnection.Endpoint, check.BucketName).Observe(listTimer.Elapsed())

	// For backward compatibility.
	// AWS SDK v2 doesn't support path with leading prefixes.
	check.ObjectPath = strings.TrimPrefix(check.ObjectPath, "/")

	data := utils.RandomString(16)
	updateTimer := NewTimer()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &check.BucketName,
		Key:    &check.ObjectPath,
		Body:   bytes.NewReader([]byte(data)),
	})
	if err != nil {
		return results.Failf("Failed to put object %s in bucket %s: %v", check.ObjectPath, check.BucketName, err)
	}
	updateHistogram.WithLabelValues(check.AWSConnection.Endpoint, check.BucketName).Observe(updateTimer.Elapsed())

	obj, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &check.BucketName,
		Key:    &check.ObjectPath,
	})
	if err != nil {
		return results.Failf("Failed to get object %s in bucket %s: %v", check.ObjectPath, check.BucketName, err)
	}

	returnedData, _ := io.ReadAll(obj.Body)
	if string(returnedData) != data {
		return results.Failf("Get object doesn't match %s != %s", data, string(returnedData))
	}

	return results
}

// nolint:staticcheck
// FIXME: deprecated global endpoint resolver
func GetAWSConfig(ctx *context.Context, conn connection.AWSConnection) (cfg aws.Config, err error) {
	var options []func(*config.LoadOptions) error

	if conn.Region != "" {
		options = append(options, config.WithRegion(conn.Region))
	}

	if conn.Endpoint != "" {
		options = append(options, config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...any) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL: conn.Endpoint,
				}, nil
			},
		)))
	}

	if !conn.AccessKey.IsEmpty() {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(conn.AccessKey.ValueStatic, conn.SecretKey.ValueStatic, "")))
	}

	if conn.SkipTLSVerify {
		var tr http.RoundTripper
		if ctx.IsTrace() {
			httplogger := &httpretty.Logger{
				Time:           true,
				TLS:            false,
				RequestHeader:  false,
				RequestBody:    false,
				ResponseHeader: true,
				ResponseBody:   false,
				Colors:         true,
				Formatters:     []httpretty.Formatter{&httpretty.JSONFormatter{}},
			}
			tr = httplogger.RoundTripper(tr)
		} else {
			tr = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}

		options = append(options, config.WithHTTPClient(&http.Client{Transport: tr}))
	}

	return config.LoadDefaultConfig(ctx, options...)
}
