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

	if check.AWSConnection == nil {
		return results.ErrorMessage(errors.New("missing AWS connection"))
	}

	connection, err := ctx.HydrateConnectionByURL(check.AWSConnection.ConnectionName)
	if err != nil {
		return results.Failf("failed to populate AWS connection: %v", err)
	} else if connection == nil {
		connection = &models.Connection{}
		connection, err = connection.Merge(ctx, check.AWSConnection)
		if err != nil {
			return results.Failf("failed to populate AWS connection: %v", err)
		}
	}

	fs, err := artifacts.GetFSForConnection(ctx.Duty(), *connection)
	if err != nil {
		return results.ErrorMessage(err)
	}

	check.Path = getS3Path(check.Path)
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

// getS3Path returns the actual path stripping of the s3:// prefix and the bucket name.
// The path is expected to be in the format "s3://bucket_name/<actual_path>"
func getS3Path(path string) string {
	trimmed := strings.TrimPrefix(path, "s3://")
	splits := strings.SplitN(trimmed, "/", 2)
	if len(splits) != 2 {
		return ""
	}

	return splits[1]
}
