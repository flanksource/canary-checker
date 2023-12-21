//go:build !fast

package checks

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
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

	connection, err := ctx.HydrateConnectionByURL(check.AWSConnection.ConnectionName)
	if err != nil {
		return results.Failf("failed to populate AWS connection: %v", err)
	} else if connection == nil {
		connection = &models.Connection{Type: models.ConnectionTypeS3}
		if check.S3Connection.Bucket == "" {
			check.S3Connection.Bucket = bucket
		}

		connection, err = connection.Merge(ctx, check.S3Connection)
		if err != nil {
			return results.Failf("failed to populate AWS connection: %v", err)
		}
	}

	fs, err := artifacts.GetFSForConnection(ctx.Duty(), *connection)
	if err != nil {
		return results.ErrorMessage(err)
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
