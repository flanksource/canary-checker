package checks

import (
	"fmt"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	bucketScanObjectCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_s3_scan_count",
			Help: "The total number of objects",
		},
		[]string{"endpoint", "bucket"},
	)
	bucketScanLastWrite = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_s3_last_write",
			Help: "The last write time",
		},
		[]string{"endpoint", "bucket"},
	)
	bucketScanTotalSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_s3_total_size",
			Help: "The total object size in bytes",
		},
		[]string{"endpoint", "bucket"},
	)
)

func init() {
	prometheus.MustRegister(bucketScanObjectCount, bucketScanLastWrite, bucketScanTotalSize)
}

type S3BucketChecker struct{}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *S3BucketChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.S3Bucket {
		for _, result := range c.Check(conf.S3BucketCheck) {
			checks = append(checks, result)
		}
	}
	return checks
}

// Type: returns checker type
func (c *S3BucketChecker) Type() string {
	return "s3_bucket"
}

func (c *S3BucketChecker) Check(bucket pkg.S3BucketCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult

	if _, err := DNSLookup(bucket.Endpoint); err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Message:  fmt.Sprintf("Failed to resolve DNS for %s", bucket.Endpoint),
			Endpoint: bucket.Bucket,
		})
		return result
	}

	cfg := aws.NewConfig().
		WithRegion(bucket.Region).
		WithEndpoint(bucket.Endpoint).
		WithCredentials(
			credentials.NewStaticCredentials(bucket.AccessKey, bucket.SecretKey, ""),
		)
	ssn, err := session.NewSession(cfg)
	if err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Message:  fmt.Sprintf("Failed to create S3 session for %s: %v", bucket.Bucket, err),
			Endpoint: bucket.Bucket,
		})
		return result
	}
	client := s3.New(ssn)
	//client.Config.S3ForcePathStyle = aws.Bool(true)

	var marker *string = nil

	var latestObject *s3.Object = nil
	var regex *regexp.Regexp = regexp.MustCompile("(.*?)")
	if bucket.ObjectPath != "" {
		re, err := regexp.Compile(bucket.ObjectPath)
		if err != nil {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Invalid:  true,
				Endpoint: bucket.Bucket,
				Message:  fmt.Sprintf("Failed to compile regex for listing objects in bucket %s: %v", bucket.Bucket, err),
			})
			return result
		}
		regex = re
	}

	for {
		req := &s3.ListObjectsInput{
			Bucket:  aws.String(bucket.Bucket),
			Marker:  marker,
			MaxKeys: aws.Int64(100),
		}
		resp, err := client.ListObjects(req)
		if err != nil {
			result = append(result, &pkg.CheckResult{
				Pass:     false,
				Message:  fmt.Sprintf("Failed to list objects in bucket %s: %v", bucket.Bucket, err),
				Endpoint: bucket.Bucket,
			})
			return result
		}

		for _, obj := range resp.Contents {
			if regex.Match([]byte(aws.StringValue(obj.Key))) {
				bucketScanTotalSize.WithLabelValues(bucket.Endpoint, bucket.Bucket).Add(float64(aws.Int64Value(obj.Size)))
				if latestObject == nil || obj.LastModified.After(aws.TimeValue(latestObject.LastModified)) {
					latestObject = obj
				}
			}
		}

		bucketScanObjectCount.WithLabelValues(bucket.Endpoint, bucket.Bucket).Add(float64(len(resp.Contents)))

		if resp.IsTruncated != nil && aws.BoolValue(resp.IsTruncated) && len(resp.Contents) > 0 {
			marker = resp.Contents[len(resp.Contents)-1].Key
		} else {
			break
		}
	}

	if latestObject == nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Endpoint: bucket.Bucket,
			Message:  fmt.Sprintf("Could not find any matching object in bucket %s", bucket.Bucket),
		})
		return result
	}

	latestObjectAge := time.Now().Sub(aws.TimeValue(latestObject.LastModified))
	bucketScanLastWrite.WithLabelValues(bucket.Endpoint, bucket.Bucket).Set(float64(latestObject.LastModified.Unix()))

	if latestObjectAge.Seconds() > float64(bucket.MaxAge) {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Message:  fmt.Sprintf("Latest object age for bucket %s is %f seconds required at most %d seconds", bucket.Bucket, latestObjectAge.Seconds(), bucket.MaxAge),
			Endpoint: bucket.Bucket,
		})
		return result
	}

	latestObjectSize := aws.Int64Value(latestObject.Size)

	if latestObjectSize < bucket.MinSize {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Endpoint: bucket.Bucket,
			Message:  fmt.Sprintf("Latst object size for bucket %s is %d bytes required at least %d bytes", bucket.Bucket, latestObjectSize, bucket.MinSize),
			Metrics:  nil,
		})
		return result
	}

	result = append(result, &pkg.CheckResult{
		Pass:     true,
		Message:  fmt.Sprintf("Successfully scaned bucket %s", bucket.Bucket),
		Endpoint: bucket.Bucket,
	})

	return result
}
