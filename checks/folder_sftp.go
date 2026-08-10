package checks

import (
	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func CheckSFTP(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := newFolderResult(ctx, check)
	results := result.ToSlice()

	if err := check.SFTPConnection.HydrateConnection(ctx); err != nil {
		return failFolder(results, folderConnectionError, "failed to populate SFTP connection: %v", err)
	}

	fs, err := artifacts.GetFSForConnection(ctx.Context, check.SFTPConnection.ToModel())
	if err != nil {
		return errorFolder(results, folderConnectionError, err)
	}

	folders, err := genericFolderCheck(ctx, fs, check.Path, check.Recursive, check.Filter)
	result.AddDetails(folders)
	if err != nil {
		return errorFolder(results, folderListingError, err)
	}

	return applyFolderTest(results, folders, check.FolderTest)
}
