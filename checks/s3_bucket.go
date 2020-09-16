package checks

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	bucketScanObjectCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
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

type S3BucketChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *S3BucketChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.S3Bucket {
		results = append(results, c.Check(conf))
	}
	return results
}

// Type: returns checker type
func (c *S3BucketChecker) Type() string {
	return "s3Bucket"
}

func (c *S3BucketChecker) Check(extConfig external.Check) *pkg.CheckResult {
	bucket := extConfig.(v1.S3BucketCheck)
	if _, err := DNSLookup(bucket.Endpoint); err != nil {
		return unexpectedErrorf(bucket, err, "failed to resolve DNS")
	}

	cfg := aws.NewConfig().
		WithRegion(bucket.Region).
		WithEndpoint(bucket.Endpoint).
		WithCredentials(
			credentials.NewStaticCredentials(bucket.AccessKey, bucket.SecretKey, ""),
		)
	if bucket.SkipTLSVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		cfg = cfg.WithHTTPClient(&http.Client{Transport: tr})
	}
	ssn, err := session.NewSession(cfg)
	if err != nil {
		return unexpectedErrorf(bucket, err, "failed to create S3 session")
	}
	client := s3.New(ssn)
	client.Config.S3ForcePathStyle = aws.Bool(bucket.UsePathStyle)

	var marker *string = nil

	var latestObject *s3.Object = nil
	var objects int
	var totalSize int64
	var regex *regexp.Regexp
	if bucket.ObjectPath != "" {
		re, err := regexp.Compile(bucket.ObjectPath)
		if err != nil {
			return unexpectedErrorf(bucket, err, "failed to compile regex: %s", bucket.ObjectPath)
		}
		regex = re
	}

	for {
		req := &s3.ListObjectsInput{
			Bucket:  aws.String(bucket.Bucket),
			Marker:  marker,
			MaxKeys: aws.Int64(500),
		}
		resp, err := client.ListObjects(req)
		if err != nil {
			return unexpectedErrorf(bucket, err, "failed to list bucket")
		}

		for _, obj := range resp.Contents {
			if regex != nil && !regex.Match([]byte(aws.StringValue(obj.Key))) {
				continue
			}
			bucketScanTotalSize.WithLabelValues(bucket.Endpoint, bucket.Bucket).Add(float64(aws.Int64Value(obj.Size)))
			if latestObject == nil || obj.LastModified.After(aws.TimeValue(latestObject.LastModified)) {
				latestObject = obj
			}

			objects++
			totalSize += *obj.Size
		}

		if resp.IsTruncated != nil && aws.BoolValue(resp.IsTruncated) && len(resp.Contents) > 0 {
			marker = resp.Contents[len(resp.Contents)-1].Key
		} else {
			break
		}
	}

	bucketScanObjectCount.WithLabelValues(bucket.Endpoint, bucket.Bucket).Set(float64(objects))

	bucketScanTotalSize.WithLabelValues(bucket.Endpoint, bucket.Bucket).Set(float64(totalSize))

	if latestObject == nil {
		return Failf(bucket, "could not find any matching objects")
	}

	latestObjectAge := time.Now().Sub(aws.TimeValue(latestObject.LastModified))
	bucketScanLastWrite.WithLabelValues(bucket.Endpoint, bucket.Bucket).Set(float64(latestObject.LastModified.Unix()))

	if latestObjectAge.Seconds() > float64(bucket.MaxAge) {
		return Failf(bucket, "Latest object age is %s required at most %s", age(latestObjectAge), age(time.Second*time.Duration(bucket.MaxAge)))
	}

	latestObjectSize := aws.Int64Value(latestObject.Size)

	if bucket.MinSize > 0 && latestObjectSize < bucket.MinSize {
		return Failf(bucket, "Latest object is %s required at least %s", mb(latestObjectSize), mb(bucket.MinSize))
	}

	return Passf(bucket, fmt.Sprintf("maxAge=%s size=%s objects=%d totalSize=%s", age(latestObjectAge), mb(latestObjectSize), objects, mb(totalSize)))
}
