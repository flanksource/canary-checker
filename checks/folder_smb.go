package checks

import (
	"fmt"
	"net"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/hirochachacha/go-smb2"
)

type SMBSession struct {
	net.Conn
	*smb2.Session
	*smb2.Share
}

func (s *SMBSession) Close() error {
	if s.Conn != nil {
		_ = s.Conn.Close()
	}
	if s.Session != nil {
		_ = s.Session.Logoff()
	}
	if s.Share != nil {
		_ = s.Share.Umount()
	}
	return nil
}

func smbConnect(server string, port int, share string, auth *v1.Authentication) (Filesystem, uint64, uint64, uint64, error) {
	var err error
	var smb *SMBSession
	server = server + ":" + fmt.Sprintf("%d", port)
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, 0, 0, 0, err
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
		return nil, 0, 0, 0, err
	}
	smb.Session = s
	fs, err := s.Mount(share)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	smb.Share = fs
	info, err := fs.Statfs(".")
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return smb, info.TotalBlockCount(), info.FreeBlockCount(), info.BlockSize(), err
}

func CheckSmb(ctx *context.Context, check v1.FolderCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	namespace := ctx.Canary.Namespace

	var serverPath = strings.TrimPrefix(check.Path, "smb://")
	server, sharename, path, err := extractServerDetails(serverPath)
	if err != nil {
		return results.ErrorMessage(err)
	}

	foundConn, err := check.SMBConnection.PopulateFromConnection(ctx, db.Gorm)
	if err != nil {
		return results.Failf("failed to populate SMB connection: %w", err)
	}

	auth := check.SMBConnection.Auth
	if !foundConn {
		auth, err = GetAuthValues(check.SMBConnection.Auth, ctx.Kommons, namespace)
		if err != nil {
			return results.ErrorMessage(err)
		}
	}

	session, totalBlockCount, freeBlockCount, blockSize, err := smbConnect(server, check.SMBConnection.GetPort(), sharename, auth)
	if err != nil {
		return results.ErrorMessage(err)
	}
	if session != nil {
		defer session.Close()
	}

	folders, err := getGenericFolderCheck(session, path, check.Filter)
	if err != nil {
		return results.ErrorMessage(err)
	}

	folders.SupportsAvailableSize = true
	folders.SupportsTotalSize = true

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
