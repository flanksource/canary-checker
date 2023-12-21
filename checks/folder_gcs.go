package checks

import (
	"errors"
	"strings"

	gcs "cloud.google.com/go/storage"
	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
)

type GCS struct {
	BucketName string
	*gcs.Client
}

func CheckGCSBucket(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if check.GCPConnection == nil {
		return results.ErrorMessage(errors.New("missing GCP connection"))
	}

	connection, err := ctx.HydrateConnectionByURL(check.GCPConnection.ConnectionName)
	if err != nil {
		return results.Failf("failed to populate GCP connection: %v", err)
	} else if connection == nil {
		connection = &models.Connection{}
		connection, err = connection.Merge(ctx, check.GCPConnection)
		if err != nil {
			return results.Failf("failed to populate GCP connection: %v", err)
		}
	}

	fs, err := artifacts.GetFSForConnection(ctx.Duty(), *connection)
	if err != nil {
		return results.ErrorMessage(err)
	}

	// check.Path will be in the format gcs://bucket_name/<actual_path>
	fullPath := getGCSBucketName(check.Path)
	splits := strings.SplitN(fullPath, "/", 2)
	if len(splits) > 1 {
		check.Path = splits[1]
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

// getGCSBucketName returns the actual path stripping of the gcs:// prefix and the bucket name.
// The path is expected to be in the format "gcs://bucket_name/<actual_path>"
func getGCSBucketName(path string) string {
	trimmed := strings.TrimPrefix(path, "gcs://")
	splits := strings.SplitN(trimmed, "/", 2)
	if len(splits) != 2 {
		return ""
	}

	return splits[1]
}
