package checks

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

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
	if err != nil {
		return results.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}
	return results
}

func getLocalFolderCheck(path string, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}
	_filter, err := filter.New()
	if err != nil {
		return nil, err
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		// directory is empty. returning duration of directory
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		return &FolderCheck{Oldest: info, Newest: info}, nil
	}

	for _, file := range files {
		if file.IsDir() || !_filter.Filter(file) {
			continue
		}

		result.Append(file)
	}
	return &result, err
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
		return &FolderCheck{Oldest: info, Newest: info}, nil
	}

	for _, file := range files {
		if file.IsDir() || !_filter.Filter(file) {
			continue
		}

		result.Append(file)
	}
	return &result, nil
}
