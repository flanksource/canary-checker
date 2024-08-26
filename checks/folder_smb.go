package checks

import (
	"fmt"
	"strings"

	"github.com/flanksource/artifacts/clients/smb"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func CheckSmb(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	var serverPath = strings.TrimPrefix(check.Path, "smb://")
	server, sharename, path, err := extractServerDetails(serverPath)
	if err != nil {
		return results.ErrorMessage(err)
	}

	err = check.SMBConnection.Populate(ctx)
	if err != nil {
		return results.Failf("failed to populate SMB connection: %v", err)
	}

	session, err := smb.SMBConnect(server, fmt.Sprintf("%d", check.SMBConnection.GetPort()), sharename, check.SMBConnection.Authentication)
	if err != nil {
		return results.ErrorMessage(err)
	}
	if session != nil {
		defer session.Close()
	}

	folders, err := genericFolderCheck(session, path, check.Recursive, check.Filter)
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
