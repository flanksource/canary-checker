package checks

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/kommons"

	"github.com/flanksource/commons/logger"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/hirochachacha/go-smb2"
)

type SmbChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *SmbChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c SmbChecker) GetClient() *kommons.Client {
	return c.kommons
}

func (c *SmbChecker) Type() string {
	return "smb"
}

func (c *SmbChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Smb {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

type SMBSession struct {
	net.Conn
	*smb2.Session
	*smb2.Share
}

type Filesystem interface {
	Close()
	ReadDir(name string) ([]os.FileInfo, error)
	Stat(name string) (os.FileInfo, error)
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

func (c *SmbChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	smbCheck := extConfig.(v1.SmbCheck)
	result := pkg.Success(smbCheck)
	namespace := ctx.Canary.Namespace

	server, sharename, path, err := getServerDetails(smbCheck.Server, 445)
	if err != nil {
		return result.ErrorMessage(err)
	}

	auth, err := GetAuthValues(smbCheck.Auth, c.kommons, namespace)
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

	folders, err := getFolderCheck(session, path)
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(smbCheck.FolderTest); test != "" {
		result.Failf(test)
	}
	return result
}

func getServerDetails(serverPath string, port int) (server, sharename, searchPath string, err error) {
	serverPath = strings.TrimLeft(serverPath, "\\")
	if serverPath == "" {
		return "", "", "", fmt.Errorf("error parsing server path")
	}
	serverDetails := strings.SplitN(serverPath, "\\", 3)
	server = fmt.Sprintf("%s:%d", serverDetails[0], port)
	switch len(serverDetails) {
	case 1:
		return "", "", "", fmt.Errorf("error parsing server path")
	case 2:
		logger.Tracef("searchPath not found in the server path. Default '.' will be taken")
		sharename = serverDetails[1]
		searchPath = "."
		return
	default:
		sharename = serverDetails[1]
		searchPath = strings.ReplaceAll(serverDetails[2], "\\", "/")
		return
	}
}

func getFolderCheck(fs Filesystem, dir string) (*FolderCheck, error) {
	result := FolderCheck{}

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
		return &FolderCheck{Oldest: timeSince(info.ModTime()), Newest: timeSince(info.ModTime())}, nil
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if result.Oldest.IsZero() || result.Oldest.Milliseconds() < timeSince(file.ModTime()).Milliseconds() {
			result.Oldest = timeSince(file.ModTime())
		}
		if result.Newest.IsZero() || result.Newest.Milliseconds() > timeSince(file.ModTime()).Milliseconds() {
			result.Newest = timeSince(file.ModTime())
		}
		result.Files = append(result.Files, file)
		if result.MaxSize < file.Size() {
			result.MaxSize = file.Size()
		}
		if result.MinSize > file.Size() {
			result.MinSize = file.Size()
		}
	}
	return &result, nil
}
