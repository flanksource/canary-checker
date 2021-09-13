package checks

import (
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/commons/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
	"github.com/prometheus/client_golang/prometheus"
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
func (c *S3BucketChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.S3Bucket {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// Type: returns checker type
func (c *S3BucketChecker) Type() string {
	return "s3Bucket"
}

type S3FileInfo struct {
	obj types.Object
}

func (obj S3FileInfo) Name() string {
	return *obj.obj.Key
}

func (obj S3FileInfo) Size() int64 {
	return obj.obj.Size
}

func (obj S3FileInfo) Mode() fs.FileMode {
	return fs.FileMode(0644)
}

func (obj S3FileInfo) ModTime() time.Time {
	return *obj.obj.LastModified
}

func (obj S3FileInfo) IsDir() bool {
	return strings.HasSuffix(obj.Name(), "/")
}

func (obj S3FileInfo) Sys() interface{} {
	return obj.obj
}

type S3 struct {
	*s3.Client
	Bucket string
}

func (conn *S3) CheckFolder(ctx *context.Context, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}

	var marker *string = nil
	parts := strings.Split(conn.Bucket, "/")
	bucket := parts[0]
	prefix := ""
	if len(parts) > 0 {
		prefix = strings.Join(parts[1:], "/")
	}
	maxKeys := 500
	for {
		logger.Debugf("%s fetching %d, prefix%s, marker=%s", bucket, maxKeys, prefix, marker)
		req := &s3.ListObjectsInput{
			Bucket:  aws.String(conn.Bucket),
			Marker:  marker,
			MaxKeys: int32(maxKeys),
			Prefix:  &prefix,
		}
		resp, err := conn.ListObjects(ctx, req)
		if err != nil {
			return nil, err
		}

		_filter, err := filter.New()
		if err != nil {
			return nil, err
		}
		for _, obj := range resp.Contents {
			file := S3FileInfo{obj}
			if !_filter.Filter(file) {
				continue
			}
			result.Append(file)
		}
		if resp.IsTruncated && len(resp.Contents) > 0 {
			marker = resp.Contents[len(resp.Contents)-1].Key
		} else {
			break
		}
	}
	// bucketScanTotalSize.WithLabelValues(bucket.Endpoint, bucket.Bucket).Add(float64(aws.Int64Value(obj.Size)))
	return &result, nil
}

func (c *S3BucketChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	bucket := extConfig.(v1.S3BucketCheck)
	result := pkg.Success(bucket, ctx.Canary)

	cfg, err := awsUtil.NewSession(ctx, bucket.AWSConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}

	client := &S3{
		Client: s3.NewFromConfig(*cfg, func(o *s3.Options) {
			o.UsePathStyle = bucket.UsePathStyle
		}),
		Bucket: bucket.Bucket,
	}
	folders, err := client.CheckFolder(ctx, bucket.Filter)
	if err != nil {
		return result.ErrorMessage(fmt.Errorf("failed to retrieve s3://%s: %v", bucket.Bucket, err))
	}
	result.AddDetails(folders)

	if test := folders.Test(bucket.FolderTest); test != "" {
		return result.Failf(test)
	}

	return result
}
