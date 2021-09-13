package checks

import (
	gcs "cloud.google.com/go/storage"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/clients/gcp"
)

type GCSBucketChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *GCSBucketChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.GCSBucket {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// Type: returns checker type
func (c *GCSBucketChecker) Type() string {
	return "gcsBucket"
}

type GCS struct {
	BucketName string
	*gcs.Client
}

func (c *GCSBucketChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.GCSBucketCheck)
	result := pkg.Success(check, ctx.Canary)
	cfg, err := gcp.NewSession(ctx, check.GCPConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}
	client := GCS{
		BucketName: check.Bucket,
		Client:     cfg,
	}
	folders, err := client.CheckFolder(ctx, check.Filter)
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(folders)
	if test := folders.Test(check.FolderTest); test != "" {
		result.Failf(test)
	}
	return result
}

func (conn *GCS) CheckFolder(ctx *context.Context, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}
	bucket := conn.Bucket(conn.BucketName)
	objs := bucket.Objects(ctx, nil)
	_filter, err := filter.New()
	if err != nil {
		return nil, err
	}
	obj, err := objs.Next()
	// empty bucket
	if obj == nil {
		return &result, nil
	}
	if err != nil {
		return nil, nil
	}
	for {
		file := gcp.GCSFileInfo{Object: obj}
		if file.IsDir() || !_filter.Filter(file) {
			continue
		}

		result.Append(file)
		obj, err = objs.Next()
		if obj == nil {
			return &result, nil
		}
		if err != nil {
			return nil, err
		}
	}
}
