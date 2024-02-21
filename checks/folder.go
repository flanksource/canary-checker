package checks

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/artifacts"
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
	path := strings.ToLower(check.Path)
	ctx = ctx.WithCheck(check)
	if ctx.CanTemplate() {
		if err := ctx.TemplateStruct(&check.Filter); err != nil {
			return pkg.Invalid(check, ctx.Canary, fmt.Sprintf("failed to template filter: %v", err))
		}
	}
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
	localFS, err := artifacts.GetFSForConnection(ctx.Context, models.Connection{Type: models.ConnectionTypeFolder})
	if err != nil {
		return results.ErrorMessage(err)
	}

	folders, err := genericFolderCheck(localFS, check.Path, check.Recursive, check.Filter)
	result.AddDetails(folders)

	if err != nil {
		return results.ErrorMessage(err)
	}

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}
	return results
}

func genericFolderCheck(dirFS artifacts.Filesystem, path string, recursive bool, filter v1.FolderFilter) (FolderCheck, error) {
	return _genericFolderCheck(true, dirFS, path, recursive, filter)
}

// genericFolderCheckWithoutPrecheck is used for those filesystems that do not support fetching the stat of a directory.
// Eg: s3, gcs.
// It will not pre check whether the given path is a directory.
func genericFolderCheckWithoutPrecheck(dirFS artifacts.Filesystem, path string, recursive bool, filter v1.FolderFilter) (FolderCheck, error) {
	return _genericFolderCheck(false, dirFS, path, recursive, filter)
}

func _genericFolderCheck(supportsDirStat bool, dirFS artifacts.Filesystem, path string, recursive bool, filter v1.FolderFilter) (FolderCheck, error) {
	result := FolderCheck{}
	_filter, err := filter.New()
	if err != nil {
		return result, err
	}
	_filter.AllowDir = recursive

	var fileInfo os.FileInfo
	if supportsDirStat {
		fileInfo, err := dirFS.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return result, nil
			}
			return result, err
		} else if !fileInfo.IsDir() {
			return result, fmt.Errorf("%s is not a directory", path)
		}
	}

	files, err := getFolderContents(dirFS, path, _filter)
	if err != nil {
		return result, err
	}

	if len(files) == 0 {
		if fileInfo == nil {
			return FolderCheck{}, nil
		}

		// directory is empty. returning duration of directory
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
func getFolderContents(dirFs artifacts.Filesystem, path string, filter *v1.FolderFilterContext) ([]fs.FileInfo, error) {
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
			continue
		}

		result = append(result, info)
		if info.IsDir() { // This excludes even directory symlinks
			subFiles, err := getFolderContents(dirFs, path+"/"+info.Name(), filter)
			if err != nil {
				return nil, err
			}

			result = append(result, subFiles...)
		}
	}

	return result, err
}
