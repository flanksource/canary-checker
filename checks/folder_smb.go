package checks

import (
	"fmt"
	"strings"

	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func CheckSmb(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	serverPath := strings.TrimPrefix(check.Path, "smb://")
	server, share, path, err := extractServerDetails(serverPath)
	if err != nil {
		return results.ErrorMessage(err)
	}

	if err := check.SMBConnection.Populate(ctx); err != nil {
		return results.Failf("failed to populate SMB connection: %v", err)
	}

	if server != "" {
		check.SMBConnection.Domain = server
	}

	if share != "" {
		check.SMBConnection.Share = share
	}

	fs, err := artifacts.GetFSForConnection(ctx.Context, check.SMBConnection.ToModel())
	if err != nil {
		return results.ErrorMessage(err)
	}

	folders, err := genericFolderCheck(ctx, fs, path, check.Recursive, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}

	var totalBlockCount, freeBlockCount, blockSize int // TODO:
	folders.AvailableSize = int64(freeBlockCount * blockSize)
	folders.TotalSize = int64(totalBlockCount * blockSize)

	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}
	return results
}

func extractServerDetails(serverPath string) (server, sharename, searchPath string, err error) {
	serverPath = strings.TrimLeft(serverPath, "\\")
	if serverPath == "" {
		return "", "", "", fmt.Errorf("empty path specified")
	}
	serverDetails := strings.SplitN(serverPath, "\\", 3)
	server = serverDetails[0]
	switch len(serverDetails) {
	case 1:
		return "", "", "", fmt.Errorf("error parsing path: %v", serverPath)
	case 2:
		sharename = serverDetails[1]
		searchPath = "."
		return
	default:
		sharename = serverDetails[1]
		searchPath = strings.ReplaceAll(serverDetails[2], "\\", "/")
		return
	}
}
