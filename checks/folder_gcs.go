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

	if check.GCSConnection == nil {
		return results.ErrorMessage(errors.New("missing GCS connection"))
	}

	var bucket string
	bucket, check.Path = parseGCSPath(check.Path)

	connection, err := ctx.HydrateConnectionByURL(check.GCPConnection.ConnectionName)
	if err != nil {
		return results.Failf("failed to populate GCS connection: %v", err)
	} else if connection == nil {
		connection = &models.Connection{Type: models.ConnectionTypeGCS}
		if check.GCSConnection.Bucket == "" {
			check.GCSConnection.Bucket = bucket
		}

		connection, err = connection.Merge(ctx, check.GCSConnection)
		if err != nil {
			return results.Failf("failed to populate GCS connection: %v", err)
		}
	}

	fs, err := artifacts.GetFSForConnection(ctx.Context, *connection)
	if err != nil {
		return results.ErrorMessage(err)
	}

	folders, err := genericFolderCheckWithoutPrecheck(ctx, fs, check.Path, check.Recursive, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}

	return results
}

// parseGCSPath returns the bucket name and the actual path stripping of the gcs:// prefix and the bucket name.
// The path is expected to be in the format "gcs://bucket_name/<actual_path>"
func parseGCSPath(fullpath string) (bucket, path string) {
	trimmed := strings.TrimPrefix(fullpath, "gcs://")
	splits := strings.SplitN(trimmed, "/", 2)
	if len(splits) != 2 {
		return splits[0], ""
	}

	return splits[0], splits[1]
}
