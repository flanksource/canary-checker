package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/utils"
	"github.com/ncw/swift"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
)

func prepareS3E2E(ctx context.Context, fixture S3Fixture) error {
	client, err := getS3Client(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get s3 client")
	}

	for _, bucket := range fixture.CreateBuckets {
		req := &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		}
		if _, err := client.CreateBucket(ctx, req); err != nil {
			return errors.Wrapf(err, "failed to create bucket %s", bucket)
		}
	}

	for _, file := range fixture.Files {
		body := utils.RandomString(int(file.Size))
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(file.Bucket),
			Key:         aws.String(file.Filename),
			Body:        bytes.NewReader([]byte(body)),
			ContentType: aws.String(file.ContentType),
			Metadata: map[string]string{
				"Last-Modified": swift.TimeToFloatString(time.Now().Add(-1 * file.Age)),
			},
		})
		if err != nil {
			return errors.Wrapf(err, "failed to put object %s to bucket %s", file.Filename, file.Bucket)
		}
	}

	return nil
}

func cleanupS3E2E(ctx context.Context, fixture S3Fixture) {
	client, err := getS3Client(ctx)
	if err != nil {
		logger.Errorf("failed to create s3 client: %v", err)
		return
	}

	for _, file := range fixture.Files {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(file.Bucket),
			Key:    aws.String(file.Filename),
		})
		if err != nil {
			logger.Errorf("failed to delete object %s in bucket %s: %v", file.Filename, file.Bucket, err)
		}
	}

	for _, bucket := range fixture.CreateBuckets {
		if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			logger.Errorf("failed to delete bucket %s: %v", bucket, err)
		}
	}
}

type S3Config struct {
	AccessKey string
	SecretKey string
	Region    string
	Endpoint  string
}

func getS3Client(ctx context.Context) (*s3.Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	s3Cfg := getS3Credentials()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(s3Cfg.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...any) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL: s3Cfg.Endpoint,
				}, nil
			},
		)),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3Cfg.AccessKey, s3Cfg.SecretKey, "")),
		config.WithHTTPClient(&http.Client{Transport: tr}),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
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

func main() {
	logger.Infof("Setting up")
	if err := prepareS3E2E(context.Background(), s3Fixtures); err != nil {
		logger.Errorf("error setting up %v", err)
	}
}

var (
	s3Fixtures = S3Fixture{
		CreateBuckets: []string{
			"tests-e2e-1",
			"tests-e2e-2",
		},
		Files: []S3FixtureFile{
			{
				Bucket:      "tests-e2e-1",
				Filename:    "pg/backups/date1/backup.zip",
				Size:        50,
				Age:         30 * 24 * time.Hour, // 30 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "pg/backups/date2/backup.zip",
				Size:        50,
				Age:         7 * 24 * time.Hour, // 7 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "mysql/backups/date1/mysql.zip",
				Size:        30,
				Age:         7*24*time.Hour - 10*time.Minute, // 30 days
				ContentType: "application/zip",
			},
		},
	}
)
