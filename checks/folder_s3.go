//go:build !fast

package checks

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/connection"
)

type S3 struct {
	*s3.Client
	Bucket string
}

func CheckS3Bucket(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if check.AWSConnection == nil {
		check.AWSConnection = &connection.AWSConnection{}
	} else if err := check.AWSConnection.Populate(ctx); err != nil {
		return results.Failf("failed to populate aws connection: %v", err)
	}

	cfg, err := awsUtil.NewSession(utils.Ptr(ctx.Duty()), *check.AWSConnection)
	if err != nil {
		return results.ErrorMessage(err)
	}

	client := &S3{
		Client: s3.NewFromConfig(*cfg, func(o *s3.Options) {
			o.UsePathStyle = check.AWSConnection.UsePathStyle
		}),
		Bucket: getS3BucketName(check.Path),
	}
	folders, err := client.CheckFolder(ctx, check.Filter)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to retrieve s3://%s: %v", getS3BucketName(check.Path), err))
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}

	return results
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
			file := awsUtil.S3FileInfo{Object: obj}
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

func getS3BucketName(bucket string) string {
	return strings.TrimPrefix(bucket, "s3://")
}
