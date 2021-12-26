package checks

import (
	"fmt"
	"net"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/hirochachacha/go-smb2"
)

type SMBSession struct {
	net.Conn
	*smb2.Session
	*smb2.Share
}

func (s *SMBSession) Close() {
	if s.Conn != nil {
		_ = s.Conn.Close()
	}
	if s.Session != nil {
		_ = s.Session.Logoff()
	}
	if s.Share != nil {
		_ = s.Share.Umount()
	}
}

func getSMBFolderCheck(fs Filesystem, dir string, filter v1.FolderFilter) (*FolderCheck, error) {
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

func smbConnect(server string, share string, auth *v1.Authentication) (Filesystem, error) {
	var err error
	var smb *SMBSession

	conn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	smb = &SMBSession{
		Conn: conn,
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     auth.GetUsername(),
			Password: auth.GetPassword(),
			Domain:   auth.GetDomain(),
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		return nil, err
	}
	smb.Session = s
	fs, err := s.Mount(share)
	if err != nil {
		return nil, err
	}
	smb.Share = fs
	return smb, nil
}

func CheckSmb(ctx *context.Context, check v1.FolderCheck) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)
	namespace := ctx.Canary.Namespace
	var server = strings.TrimPrefix(check.Path, "smb://")
	server, sharename, path, err := getServerDetails(server)
	if err != nil {
		return result.ErrorMessage(err)
	}

	auth, err := GetAuthValues(check.Authentication, ctx.Kommons, namespace)
	if err != nil {
		return result.ErrorMessage(err)
	}

	session, err := smbConnect(server, sharename, auth)
	if err != nil {
		return result.ErrorMessage(err)
	}
	if session != nil {
		defer session.Close()
	}

	folders, err := getSMBFolderCheck(session, path, check.Filter)
	if err != nil {
		return result.ErrorMessage(err)
	}

	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return result.Failf(test)
	}
	return result
}

func getServerDetails(serverPath string) (server, sharename, searchPath string, err error) {
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
