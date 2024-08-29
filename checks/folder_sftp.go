package checks

import (
	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func CheckSFTP(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if err := check.SFTPConnection.HydrateConnection(ctx); err != nil {
		return results.Failf("failed to populate SFTP connection: %v", err)
	}

	fs, err := artifacts.GetFSForConnection(ctx.Context, check.SFTPConnection.ToModel())
	if err != nil {
		return results.ErrorMessage(err)
	}

	folders, err := genericFolderCheck(fs, check.Path, check.Recursive, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}

	return results
}
