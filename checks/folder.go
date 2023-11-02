package checks

import (
	"fmt"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
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
	folders, err := getLocalFolderCheck(check.Path, check.Filter)
	result.AddDetails(folders)

	if err != nil {
		return results.ErrorMessage(err)
	}

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}
	return results
}

func getLocalFolderCheck(path string, filter v1.FolderFilter) (FolderCheck, error) {
	result := FolderCheck{}
	_filter, err := filter.New()
	if err != nil {
		return result, err
	}
	if dir, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	} else if !dir.IsDir() {
		return result, fmt.Errorf("%s is not a directory", path)
	}
	files, err := os.ReadDir(path)
	if err != nil {
		return result, err
	}
	if len(files) == 0 {
		// directory is empty. returning duration of directory
		info, err := os.Stat(path)
		if err != nil {
			return result, err
		}
		return FolderCheck{
			Oldest:        newFile(info),
			Newest:        newFile(info),
			AvailableSize: SizeNotSupported,
			TotalSize:     SizeNotSupported}, nil
	}

	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			return result, err
		}
		if file.IsDir() || !_filter.Filter(info) {
			continue
		}

		result.Append(info)
	}
	return result, err
}

func getGenericFolderCheck(fs Filesystem, dir string, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}
	_filter, err := filter.New()
	if err != nil {
		return nil, err
	}
	files, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		// directory is empty. returning duration of directory
		info, err := fs.Stat(dir)
		if err != nil {
			return nil, err
		}
		return &FolderCheck{
			Oldest:        newFile(info),
			Newest:        newFile(info),
			AvailableSize: SizeNotSupported,
			TotalSize:     SizeNotSupported}, nil
	}

	for _, file := range files {
		if file.IsDir() || !_filter.Filter(file) {
			continue
		}

		result.Append(file)
	}
	return &result, nil
}
