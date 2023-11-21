package checks

import (
	gocontext "context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"

	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func GetFSForConnection(c models.Connection) (connection.FilesystemRW, error) {
	switch c.Type {
	case models.ConnectionTypeAWS:
		// TODO: Implement

	case models.ConnectionTypeGCP:
		// TODO: Implement

	case models.ConnectionTypeSFTP:
		parsedURL, err := url.Parse(c.URL)
		if err != nil {
			return nil, err
		}

		client, err := sshConnect(parsedURL.Host, c.Username, c.Password)
		if err != nil {
			return nil, err
		}
		return client, nil

	case models.ConnectionTypeSMB:
		port := c.Properties["port"]
		share := c.Properties["share"]
		return smbConnect(c.URL, port, share, connection.Authentication{
			Username: types.EnvVar{ValueStatic: c.Username},
			Password: types.EnvVar{ValueStatic: c.Password},
		})
	}

	return nil, nil
}

type SMBSession struct {
	net.Conn
	*smb2.Session
	*smb2.Share
}

func (s *SMBSession) Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error) {
	return s.Share.Open(fileID)
}

func (s *SMBSession) Write(ctx gocontext.Context, path string, data []byte) (os.FileInfo, error) {
	f, err := s.Share.Create(path)
	if err != nil {
		return nil, err
	}

	if _, err = f.Write(data); err != nil {
		return nil, err
	}

	return f.Stat()
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

func smbConnect(server string, port, share string, auth connection.Authentication) (connection.FilesystemRW, error) {
	var err error
	var smb *SMBSession
	server = server + ":" + port
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

	return smb, err
}

type sshFS struct {
	*sftp.Client
}

func (s *sshFS) Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error) {
	return s.Client.Open(fileID)
}

func (s *sshFS) Write(ctx gocontext.Context, path string, data []byte) (os.FileInfo, error) {
	f, err := s.Client.Create(path)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	if _, err = f.Write(data); err != nil {
		return nil, fmt.Errorf("error writing to file: %w", err)
	}

	return f.Stat()
}

func sshConnect(host, user, password string) (*sshFS, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return &sshFS{client}, err
}
