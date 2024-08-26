package checks

import (
	"fmt"

	"github.com/flanksource/artifacts"
	"github.com/flanksource/artifacts/clients/sftp"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func CheckSFTP(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	err := check.SFTPConnection.HydrateConnection(ctx)
	if err != nil {
		return results.Failf("failed to populate SFTP connection: %v", err)
	}

	client, err := sftp.SSHConnect(fmt.Sprintf("%s:%d", check.SFTPConnection.Host, check.SFTPConnection.GetPort()), check.SFTPConnection.GetUsername(), check.SFTPConnection.GetPassword())
	if err != nil {
		return results.ErrorMessage(err)
	}
	defer client.Close()

	session := artifacts.Filesystem(client)
	folders, err := genericFolderCheck(session, check.Path, check.Recursive, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}

	return results
}
