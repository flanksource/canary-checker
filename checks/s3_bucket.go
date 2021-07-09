package checks

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/flanksource/commons/text"

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

type BucketStatus struct {
	latestObjectAge  time.Duration
	latestObjectSize int64
	objects          int
	totalSize        int64
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
	start := time.Now()
	bucket := extConfig.(v1.S3BucketCheck)
	var textResults bool
	var bucketStatus BucketStatus
	if bucket.GetDisplayTemplate() != "" {
		textResults = true
	}
	template := bucket.GetDisplayTemplate()
	if _, err := DNSLookup(bucket.Endpoint); err != nil {
		return s3BucketFailF(bucket, textResults, bucketStatus, template, "failed to resolve DNS: %v", err)
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
		return s3BucketFailF(bucket, textResults, bucketStatus, template, "failed to create S3 session: %v", err)
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
			return s3BucketFailF(bucket, textResults, bucketStatus, template, "failed to compile regex: %s ", bucket.ObjectPath, err)
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
			return s3BucketFailF(bucket, textResults, bucketStatus, template, "failed to list buckets %v", err)
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
			bucketStatus.objects = objects
			bucketStatus.totalSize = totalSize
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
		return s3BucketFailF(bucket, textResults, bucketStatus, template, "could not find any matching objects")
	}

	latestObjectAge := time.Since(aws.TimeValue(latestObject.LastModified))
	bucketScanLastWrite.WithLabelValues(bucket.Endpoint, bucket.Bucket).Set(float64(latestObject.LastModified.Unix()))
	latestObjectSize := aws.Int64Value(latestObject.Size)
	bucketStatus.latestObjectSize = latestObjectSize
	bucketStatus.latestObjectAge = latestObjectAge

	if latestObjectAge.Seconds() > float64(bucket.MaxAge) {
		return s3BucketFailF(bucket, textResults, bucketStatus, template, "Latest object age is %s required at most %s", age(latestObjectAge), age(time.Second*time.Duration(bucket.MaxAge)))
	}

	if bucket.MinSize > 0 && latestObjectSize < bucket.MinSize {
		return s3BucketFailF(bucket, textResults, bucketStatus, template, "Latest object is %s required at least %s", mb(latestObjectSize), mb(bucket.MinSize))
	}
	// Don't use s3BucketTemplateResult since we also need to check if templating succeeds here if not we fail
	var results = map[string]string{"maxAge": age(latestObjectAge), "size": mb(latestObjectSize), "count": strconv.Itoa(objects), "totalSize": mb(totalSize)}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		return s3BucketFailF(bucket, textResults, bucketStatus, template, "error templating")
	}
	return Successf(bucket, start, textResults, message)
}

func s3BucketFailF(check external.Check, textResults bool, bucketState BucketStatus, template, msg string, args ...interface{}) *pkg.CheckResult {
	message := s3BucketTemplateResult(template, bucketState.latestObjectAge, bucketState.latestObjectSize, bucketState.objects, bucketState.totalSize)
	message = message + "\n" + fmt.Sprintf(msg, args...)
	return TextFailf(check, textResults, message)
}

func s3BucketTemplateResult(template string, latestObjectAge time.Duration, latestObjectSize int64, objects int, totalSize int64) (message string) {
	var results = map[string]string{"maxAge": age(latestObjectAge), "size": mb(latestObjectSize), "count": strconv.Itoa(objects), "totalSize": mb(totalSize)}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		message = message + "\n" + err.Error()
	}
	return message
}
