package checks

import (
	"strconv"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func CheckSFTP(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	auth, err := GetAuthValues(check.SFTPConnection.Auth, ctx.Kommons, ctx.Canary.Namespace)
	if err != nil {
		return results.ErrorMessage(err)
	}
	conn, err := sshConnect(check.SFTPConnection.Host, check.SFTPConnection.GetPort(), auth.GetUsername(), auth.GetPassword())
	if err != nil {
		return results.ErrorMessage(err)
	}

	defer conn.Close()

	client, err := sftp.NewClient(conn)
	if err != nil {
		return results.ErrorMessage(err)
	}
	defer client.Close()

	session := Filesystem(client)
	folders, err := getGenericFolderCheck(session, check.Path, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return results.Failf(test)
	}
	return results
}

func sshConnect(host string, port int, user string, password string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", host+":"+strconv.Itoa(port), config)
	if err != nil {
		return nil, err
	}
	return client, nil
}
