package checks

import (
	gocontext "context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	gcs "cloud.google.com/go/storage"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
	gcpUtil "github.com/flanksource/canary-checker/pkg/clients/gcp"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Filesystem interface {
	Close() error
	ReadDir(name string) ([]os.FileInfo, error)
	Stat(name string) (os.FileInfo, error)
}

type FilesystemRW interface {
	Filesystem
	Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error)
	Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error)
}

// s3FS implements FilesystemRW for S3
type s3FS struct {
	*s3.Client
	Bucket string
}

func newS3FS(ctx context.Context, bucket string, conn connection.AWSConnection) (*s3FS, error) {
	cfg, err := awsUtil.NewSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	client := &s3FS{
		Client: s3.NewFromConfig(*cfg, func(o *s3.Options) {
			o.UsePathStyle = conn.UsePathStyle
		}),
		Bucket: strings.TrimPrefix(bucket, "s3://"),
	}

	return client, nil
}

func (t *s3FS) Close() error {
	return nil // NOOP
}

func (t *s3FS) ReadDir(name string) ([]os.FileInfo, error) {
	req := &s3.ListObjectsInput{
		Bucket: aws.String(name),
	}
	resp, err := t.Client.ListObjects(gocontext.TODO(), req)
	if err != nil {
		return nil, err
	}

	var output []os.FileInfo
	for _, r := range resp.Contents {
		output = append(output, &awsUtil.S3FileInfo{Object: r})
	}

	return output, nil
}

func (t *s3FS) Stat(path string) (os.FileInfo, error) {
	headObject, err := t.Client.HeadObject(gocontext.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}

	fileInfo := &awsUtil.S3FileInfo{
		Object: s3Types.Object{
			Key:          utils.Ptr(filepath.Base(path)),
			Size:         headObject.ContentLength,
			LastModified: headObject.LastModified,
			ETag:         headObject.ETag,
		},
	}

	return fileInfo, nil
}

func (t *s3FS) Read(ctx gocontext.Context, key string) (io.ReadCloser, error) {
	results, err := t.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer results.Body.Close()

	return results.Body, nil
}

func (t *s3FS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	_, err := t.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(path),
		Body:   data,
	})

	if err != nil {
		return nil, err
	}

	return t.Stat(path)
}

// gcsFS implements FilesystemRW for Google Cloud Storage
type gcsFS struct {
	*gcs.Client
	Bucket string
}

func newGCSFS(ctx context.Context, bucket string, conn *connection.GCPConnection) (*gcsFS, error) {
	cfg, err := gcpUtil.NewSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	client := gcsFS{
		Bucket: strings.TrimPrefix(bucket, "gcs://"),
		Client: cfg,
	}

	return &client, nil
}

func (t *gcsFS) Close() error {
	return t.Client.Close()
}

// TODO: implement
func (t *gcsFS) ReadDir(name string) ([]os.FileInfo, error) {
	return nil, nil
}

func (t *gcsFS) Stat(path string) (os.FileInfo, error) {
	obj := t.Client.Bucket(t.Bucket).Object(path)
	attrs, err := obj.Attrs(gocontext.TODO())
	if err != nil {
		return nil, err
	}

	fileInfo := &gcpUtil.GCSFileInfo{
		Object: attrs,
	}

	return fileInfo, nil
}

func (t *gcsFS) Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error) {
	obj := t.Client.Bucket(t.Bucket).Object(fileID)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (t *gcsFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	obj := t.Client.Bucket(t.Bucket).Object(path)

	content, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	writer := obj.NewWriter(ctx)
	if _, err := writer.Write(content); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return t.Stat(path)
}

// localFS implements FilesystemRW for local filesystem
type localFS struct {
	base string
}

func newLocalFS(base string) *localFS {
	return &localFS{base: base}
}

func (t *localFS) Close() error {
	return nil
}

func (t *localFS) ReadDir(name string) ([]os.FileInfo, error) {
	return nil, nil // TODO:
}

func (t *localFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(t.base, name))
}

func (t *localFS) Read(ctx gocontext.Context, path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(t.base, path))
}

func (t *localFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	fullpath := filepath.Join(t.base, path)

	// Ensure the directory exists
	err := os.MkdirAll(filepath.Dir(fullpath), os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error creating base directory: %w", err)
	}

	f, err := os.Create(fullpath)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(f, data)
	if err != nil {
		return nil, err
	}

	return t.Stat(path)
}

func GetFSForConnection(ctx context.Context, c models.Connection) (FilesystemRW, error) {
	switch c.Type {
	case models.ConnectionTypeFolder:
		path := c.Properties["path"]
		return newLocalFS(path), nil

	case models.ConnectionTypeAWS:
		bucket := c.Properties["bucket"]
		conn := connection.AWSConnection{
			ConnectionName: c.ID.String(),
		}
		if err := conn.Populate(ctx); err != nil {
			return nil, err
		}

		return newS3FS(ctx, bucket, conn)

	case models.ConnectionTypeGCP:
		bucket := c.Properties["bucket"]
		conn := &connection.GCPConnection{
			ConnectionName: c.ID.String(),
		}
		if err := conn.HydrateConnection(ctx); err != nil {
			return nil, err
		}

		client, err := newGCSFS(ctx, bucket, conn)
		if err != nil {
			return nil, err
		}
		return client, nil

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

func (s *SMBSession) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	f, err := s.Share.Create(path)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(f, data)
	if err != nil {
		return nil, fmt.Errorf("error writing file: %w", err)
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

func smbConnect(server string, port, share string, auth connection.Authentication) (FilesystemRW, error) {
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

func (s *sshFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	err := s.Client.MkdirAll(dir)
	if err != nil {
		return nil, fmt.Errorf("error creating directory: %w", err)
	}

	f, err := s.Client.Create(path)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	_, err = io.Copy(f, data)
	if err != nil {
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

	return &sshFS{Client: client}, err
}
