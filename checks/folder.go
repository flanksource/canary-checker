package checks

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/flanksource/artifacts"
	artifactFS "github.com/flanksource/artifacts/fs"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
)

const SizeNotSupported = -1

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

type FolderChecker struct {
}

func (c *FolderChecker) Type() string {
	return "folder"
}

func (c *FolderChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Folder {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *FolderChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.FolderCheck)
	ctx = ctx.WithCheck(check)
	if ctx.CanTemplate() {
		if err := ctx.TemplateStruct(&check); err != nil {
			return pkg.Invalid(check, ctx.Canary, fmt.Sprintf("failed to template check: %v", err))
		}
	}
	path := strings.ToLower(check.Path)
	if ctx.IsDebug() {
		ctx.Infof("Checking %s with filter(%s)", path, check.Filter)
	}
	switch {
	case strings.HasPrefix(path, "s3://"):
		return CheckS3Bucket(ctx, check)
	case strings.HasPrefix(path, "gcs://"):
		return CheckGCSBucket(ctx, check)
	case strings.HasPrefix(path, "smb://") || strings.HasPrefix(path, `\\`):
		return CheckSmb(ctx, check)
	case check.SFTPConnection != nil:
		return CheckSFTP(ctx, check)
	default:
		return checkLocalFolder(ctx, check)
	}
}

func checkLocalFolder(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	// Form a dummy connection to get a local filesystem
	localFS, err := artifacts.GetFSForConnection(ctx.Context, models.Connection{
		Type: models.ConnectionTypeFolder,
	})
	if err != nil {
		return results.ErrorMessage(err)
	}

	folders, err := genericFolderCheck(ctx, localFS, check.Path, check.Recursive, check.Filter)
	result.AddDetails(folders)

	if err != nil {
		return results.ErrorMessage(err)
	}

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf("%s", test)
	}
	return results
}

func genericFolderCheck(ctx *context.Context, dirFS artifactFS.Filesystem, path string, recursive bool, filter v1.FolderFilter) (FolderCheck, error) {
	result := FolderCheck{}
	_filter, err := filter.New()
	if err != nil {
		return result, err
	}
	_filter.AllowDir = recursive

	files, err := getFolderContents(ctx, dirFS, path, _filter)
	if os.IsNotExist(err) {
		return result, nil
	} else if err != nil {
		return result, err
	}

	if len(files) == 0 {
		fileInfo, err := dirFS.Stat(path)
		if err != nil {
			return result, err
		}

		if fileInfo == nil {
			return FolderCheck{}, nil
		}

		// listing is empty, returning duration of directory
		return FolderCheck{
			Oldest:        newFile(fileInfo),
			Newest:        newFile(fileInfo),
			AvailableSize: SizeNotSupported,
			TotalSize:     SizeNotSupported}, nil
	}

	for _, file := range files {
		result.Append(file)
	}

	return result, err
}

// getFolderContents walks the folder and returns all files.
// Also supports recursively fetching contents
func getFolderContents(ctx *context.Context, dirFs artifactFS.Filesystem, path string, filter *v1.FolderFilterContext) ([]fs.FileInfo, error) {
	files, err := dirFs.ReadDir(path)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, nil
	}

	var result []fs.FileInfo
	for _, info := range files {
		if !filter.Filter(info) {
			ctx.Logger.V(3).Infof("skipping %s, does not match filter", info.Name())
			continue
		}

		result = append(result, info)
	}

	return result, err
}
