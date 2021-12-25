package checks

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	gcs "cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
	"github.com/flanksource/canary-checker/pkg/clients/gcp"
	"github.com/hirochachacha/go-smb2"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/flanksource/commons/logger"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	bucketScanObjectCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_s3_scan_count",
			Help: "The total number of objects",
		},
		[]string{"endpoint", "bucket"},
	)
	bucketScanLastWrite = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_s3_last_write",
			Help: "The last write time",
		},
		[]string{"endpoint", "bucket"},
	)
	bucketScanTotalSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_s3_total_size",
			Help: "The total object size in bytes",
		},
		[]string{"endpoint", "bucket"},
	)
)

func init() {
	prometheus.MustRegister(bucketScanObjectCount, bucketScanLastWrite, bucketScanTotalSize)
}

type S3 struct {
	*s3.Client
	Bucket string
}

type GCS struct {
	BucketName string
	*gcs.Client
}

type FolderChecker struct {
}

func (c *FolderChecker) Type() string {
	return "folder"
}

func (c *FolderChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Folder {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *FolderChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.FolderCheck)
	path := strings.ToLower(check.Path)
	switch {
	case strings.HasPrefix(path, "s3://"):
		return CheckS3Bucket(ctx, check)
	case strings.HasPrefix(path, "gcs://"):
		return CheckGCSBucket(ctx, check)
	case strings.HasPrefix(path, "smb://") || strings.HasPrefix(path, `\\`):
		return CheckSmb(ctx, check)
	default:
		return checkLocalFolder(ctx, check)
	}
}

func (conn *S3) CheckFolder(ctx *context.Context, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}

	var marker *string = nil
	parts := strings.Split(conn.Bucket, "/")
	bucket := parts[0]
	prefix := ""
	if len(parts) > 0 {
		prefix = strings.Join(parts[1:], "/")
	}
	maxKeys := 500
	for {
		logger.Debugf("%s fetching %d, prefix%s, marker=%s", bucket, maxKeys, prefix, marker)
		req := &s3.ListObjectsInput{
			Bucket:  aws.String(conn.Bucket),
			Marker:  marker,
			MaxKeys: int32(maxKeys),
			Prefix:  &prefix,
		}
		resp, err := conn.ListObjects(ctx, req)
		if err != nil {
			return nil, err
		}

		_filter, err := filter.New()
		if err != nil {
			return nil, err
		}
		for _, obj := range resp.Contents {
			file := awsUtil.S3FileInfo{Object: obj}
			if !_filter.Filter(file) {
				continue
			}
			result.Append(file)
		}
		if resp.IsTruncated && len(resp.Contents) > 0 {
			marker = resp.Contents[len(resp.Contents)-1].Key
		} else {
			break
		}
	}
	// bucketScanTotalSize.WithLabelValues(bucket.Endpoint, bucket.Bucket).Add(float64(aws.Int64Value(obj.Size)))
	return &result, nil
}

func CheckS3Bucket(ctx *context.Context, check v1.FolderCheck) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)

	cfg, err := awsUtil.NewSession(ctx, *check.AWSConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}

	client := &S3{
		Client: s3.NewFromConfig(*cfg, func(o *s3.Options) {
			o.UsePathStyle = check.AWSConnection.UsePathStyle
		}),
		Bucket: getS3BucketName(check.Path),
	}
	folders, err := client.CheckFolder(ctx, check.Filter)
	if err != nil {
		return result.ErrorMessage(fmt.Errorf("failed to retrieve s3://%s: %v", getS3BucketName(check.Path), err))
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return result.Failf(test)
	}

	return result
}

func getS3BucketName(bucket string) string {
	return strings.TrimPrefix(bucket, "s3://")
}

func CheckGCSBucket(ctx *context.Context, check v1.FolderCheck) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)
	cfg, err := gcp.NewSession(ctx, *check.GCPConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}
	client := GCS{
		BucketName: getGCSBucketName(check.Path),
		Client:     cfg,
	}
	folders, err := client.CheckFolder(ctx, check.Filter)
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(folders)
	if test := folders.Test(check.FolderTest); test != "" {
		result.Failf(test)
	}
	return result
}

func (conn *GCS) CheckFolder(ctx *context.Context, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}
	bucket := conn.Bucket(conn.BucketName)
	objs := bucket.Objects(ctx, nil)
	_filter, err := filter.New()
	if err != nil {
		return nil, err
	}
	obj, err := objs.Next()
	// empty bucket
	if obj == nil {
		return &result, nil
	}
	if err != nil {
		return nil, nil
	}
	for {
		file := gcp.GCSFileInfo{Object: obj}
		if file.IsDir() || !_filter.Filter(file) {
			continue
		}

		result.Append(file)
		obj, err = objs.Next()
		if obj == nil {
			return &result, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func getGCSBucketName(bucket string) string {
	return strings.TrimPrefix(bucket, "gcs://")
}

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
	if check.SMBConnection == nil {
		return result.Failf("SMB Connection properties not defined")
	}
	server, sharename, path, err := getServerDetails(server, check.SMBConnection.GetPort())
	if err != nil {
		return result.ErrorMessage(err)
	}

	auth, err := GetAuthValues(check.SMBConnection.Auth, ctx.Kommons, namespace)
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

func checkLocalFolder(ctx *context.Context, check v1.FolderCheck) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)
	folders, err := getLocalFolderCheck(check.Path, check.Filter)
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(folders)

	if test := folders.Test(check.FolderTest); test != "" {
		return result.Failf(test)
	}
	return result
}

func getLocalFolderCheck(path string, filter v1.FolderFilter) (*FolderCheck, error) {
	result := FolderCheck{}
	_filter, err := filter.New()
	if err != nil {
		return nil, err
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		// directory is empty. returning duration of directory
		info, err := os.Stat(path)
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
	return &result, err
}
