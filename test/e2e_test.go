package test

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ncw/swift"

	"github.com/flanksource/commons/utils"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/flanksource/canary-checker/pkg"
)

var (
	s3Fixtures = S3Fixture{
		CreateBuckets: []string{
			"tests-e2e-1",
			"tests-e2e-2",
		},
		Files: []S3FixtureFile{
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/pg/backups/date1/backup.zip",
				Size:        50,
				Age:         30 * 24 * time.Hour, // 30 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/pg/backups/date2/backup.zip",
				Size:        50,
				Age:         7 * 24 * time.Hour, // 7 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/mysql/backups/date1/mysql.zip",
				Size:        30,
				Age:         7*24*time.Hour - 10*time.Minute, // 30 days
				ContentType: "application/zip",
			},
		},
	}
)

func TestE2E(t *testing.T) {
	if err := prepareS3E2E(s3Fixtures); err != nil {
		t.Errorf("s3 prepare failed: %v", err)
	}
	defer cleanupS3E2E(s3Fixtures)

	tests := []test{
		{
			name: "s3_bucket_pass",
			args: args{
				pkg.ParseConfig("../fixtures/s3_bucket_pass.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "tests-e2e-1",
					//Message:  "maxAge=0.0m size=50B objects=2 totalSize=100B",
					Metrics: []pkg.Metric{},
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "tests-e2e-1",
					//Message:  "maxAge=0.0m size=30B objects=1 totalSize=30B",
					Metrics: []pkg.Metric{},
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "tests-e2e-1",
					//Message:  "maxAge=0.0m size=50B objects=2 totalSize=100B",
					Metrics: []pkg.Metric{},
				},
			},
		},
		{
			name: "s3_bucket_fail",
			args: args{
				pkg.ParseConfig("../fixtures/s3_bucket_fail.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "tests-e2e-1",
					Message:  "Latest object is 30 bytes required at least 100 bytes",
					Metrics:  []pkg.Metric{},
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "tests-e2e-1",
					Message:  "Latest object is 50 bytes required at least 100 bytes",
					Metrics:  []pkg.Metric{},
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "tests-e2e-2",
					Message:  "could not find any matching objects",
					Metrics:  []pkg.Metric{},
				},
			},
		},
		{
			name: "docker_push_pass",
			args: args{
				pkg.ParseConfig("../fixtures/docker_push_pass.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "ttl.sh/flanksource-busybox:1.30",
					Message:  "Image ttl.sh/flanksource-busybox:1.30 successfully pushed",
					Metrics:  []pkg.Metric{},
				},
			},
		},
		{
			name: "docker_push_fail",
			args: args{
				pkg.ParseConfig("../fixtures/docker_push_fail.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "docker.io/flanksource/busybox:1.30",
					Message:  "unauthorized: incorrect username or password",
					Metrics:  []pkg.Metric{},
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "ttl.sh/flanksource-busybox:not-found-tag",
					Message:  "tag does not exist: ttl.sh/flanksource-busybox:not-found-tag",
					Metrics:  []pkg.Metric{},
				},
			},
		},
	}

	runTests(t, tests)
}

func prepareS3E2E(fixture S3Fixture) error {
	client, err := getS3Client()
	if err != nil {
		return errors.Wrap(err, "failed to get s3 client")
	}

	for _, bucket := range fixture.CreateBuckets {
		req := &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		}
		if _, err := client.CreateBucket(req); err != nil {
			return errors.Wrapf(err, "failed to create bucket %s", bucket)
		}
	}

	for _, file := range fixture.Files {
		body := utils.RandomString(int(file.Size))
		_, err = client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(file.Bucket),
			Key:         aws.String(file.Filename),
			Body:        bytes.NewReader([]byte(body)),
			ContentType: aws.String(file.ContentType),
			Metadata: map[string]*string{
				"Last-Modified": aws.String(swift.TimeToFloatString(time.Now().Add(-1 * file.Age))),
			},
		})
		if err != nil {
			return errors.Wrapf(err, "failed to put object %s to bucket %s", file.Filename, file.Bucket)
		}
	}
	return nil
}

func cleanupS3E2E(fixture S3Fixture) {
	client, err := getS3Client()
	if err != nil {
		log.Errorf("failed to create s3 client: %v", err)
		return
	}

	for _, file := range fixture.Files {
		_, err := client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(file.Bucket),
			Key:    aws.String(file.Filename),
		})
		if err != nil {
			log.Errorf("failed to delete object %s in bucket %s: %v", file.Filename, file.Bucket, err)
		}
	}

	for _, bucket := range fixture.CreateBuckets {
		if _, err := client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			log.Errorf("failed to delete bucket %s: %v", bucket, err)
		}
	}
}

type S3Config struct {
	AccessKey string
	SecretKey string
	Region    string
	Endpoint  string
}

func getS3Client() (*s3.S3, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	s3Cfg := getS3Credentials()
	cfg := aws.NewConfig().
		WithRegion(s3Cfg.Region).
		WithEndpoint(s3Cfg.Endpoint).
		WithCredentials(
			credentials.NewStaticCredentials(s3Cfg.AccessKey, s3Cfg.SecretKey, ""),
		).
		WithHTTPClient(&http.Client{Transport: tr})
	ssn, err := session.NewSession(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create s3 session")
	}
	client := s3.New(ssn)
	client.Config.S3ForcePathStyle = aws.Bool(true)
	return client, nil
}

func getS3Credentials() S3Config {
	cfg := S3Config{
		AccessKey: getEnvOrDefault("S3_ACCESS_KEY", "minio"),
		SecretKey: getEnvOrDefault("S3_SECRET_KEY", "minio123"),
		Region:    getEnvOrDefault("S3_REGION", "minio"),
		Endpoint:  getEnvOrDefault("S3_ENDPOINT", "https://minio.127.0.0.1.nip.io"),
	}
	return cfg
}

type S3Fixture struct {
	CreateBuckets []string
	Files         []S3FixtureFile
}

type S3FixtureFile struct {
	Bucket      string
	Filename    string
	Size        int64
	Age         time.Duration
	ContentType string
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}
