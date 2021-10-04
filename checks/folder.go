package checks

import (
	"io/ioutil"
	"os"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type FolderChecker struct {
}

func (c *FolderChecker) Type() string {
	return "folder"
}

func (c *FolderChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Folder {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *FolderChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.FolderCheck)
	result := pkg.Success(check, ctx.Canary)
	folders, err := getFolderCheck(check.Path, check.Filter)
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return result.Failf(test)
	}
	return result
}

func getFolderCheck(path string, filter v1.FolderFilter) (*FolderCheck, error) {
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
