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
	result := newFolderResult(ctx, check)
	results := result.ToSlice()

	serverPath := strings.TrimPrefix(check.Path, "smb://")
	server, share, path, err := extractServerDetails(serverPath)
	if err != nil {
		return errorFolder(results, folderConnectionError, err)
	}

	if err := check.SMBConnection.Populate(ctx); err != nil {
		return failFolder(results, folderConnectionError, "failed to populate SMB connection: %v", err)
	}

	if server != "" {
		check.SMBConnection.Domain = server
	}

	if share != "" {
		check.SMBConnection.Share = share
	}

	fs, err := artifacts.GetFSForConnection(ctx.Context, check.SMBConnection.ToModel())
	if err != nil {
		return errorFolder(results, folderConnectionError, err)
	}

	folders, err := genericFolderCheck(ctx, fs, path, check.Recursive, check.Filter)
	result.AddDetails(folders)

	if err != nil {
		return errorFolder(results, folderListingError, err)
	}

	return applyFolderTest(results, folders, check.FolderTest)
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
