//go:build !fast

package checks

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/flanksource/artifacts"
	artifactFS "github.com/flanksource/artifacts/fs"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type S3 struct {
	*s3.Client
	Bucket string
}

func CheckS3Bucket(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if check.S3Connection == nil {
		return results.ErrorMessage(errors.New("missing AWS connection"))
	}

	var bucket string
	bucket, check.Path = parseS3Path(check.Path)

	if err := check.S3Connection.Populate(ctx); err != nil {
		return results.ErrorMessage(err)
	}

	conn := check.S3Connection.ToModel()
	conn.SetProperty("bucket", bucket)

	fs, err := artifacts.GetFSForConnection(ctx.Context, conn)
	if err != nil {
		return results.ErrorMessage(err)
	}

	if limitFS, ok := fs.(artifactFS.ListItemLimiter); ok {
		limitFS.SetMaxListItems(ctx.Properties().Int("s3.list.max-objects", 50_000))
	}

	folders, err := genericFolderCheckWithoutPrecheck(fs, check.Path, check.Recursive, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}

	return results
}

// parseS3Path returns the bucket name and the actual path stripping of the s3:// prefix and the bucket name.
// The path is expected to be in the format "s3://bucket_name/<actual_path>"
func parseS3Path(fullpath string) (bucket, path string) {
	trimmed := strings.TrimPrefix(fullpath, "s3://")
	splits := strings.SplitN(trimmed, "/", 2)
	if len(splits) != 2 {
		return splits[0], ""
	}

	return splits[0], splits[1]
}
