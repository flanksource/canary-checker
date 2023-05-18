//go:build !fast

package aws

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/henvic/httpretty"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	v1 "github.com/flanksource/canary-checker/api/v1"
)

func NewSession(ctx *context.Context, conn v1.AWSConnection) (*aws.Config, error) {
	var tr http.RoundTripper
	tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: conn.SkipTLSVerify},
	}

	if ctx.IsTrace() {
		logger := &httpretty.Logger{
			Time:           true,
			TLS:            true,
			RequestHeader:  true,
			RequestBody:    true,
			ResponseHeader: true,
			ResponseBody:   true,
			Colors:         true, // erase line if you don't like colors
			Formatters:     []httpretty.Formatter{&httpretty.JSONFormatter{}},
		}
		tr = logger.RoundTripper(tr)
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithHTTPClient(&http.Client{Transport: tr}))

	if !conn.AccessKey.IsEmpty() {
		accessKey, err := ctx.GetEnvValueFromCache(conn.AccessKey)
		if err != nil {
			return nil, fmt.Errorf("could not parse EC2 access key: %v", err)
		}
		secretKey, err := ctx.GetEnvValueFromCache(conn.SecretKey)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Could not parse EC2 secret key: %v", err))
		}

		cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	}
	if conn.Region != "" {
		cfg.Region = conn.Region
	}
	if conn.Endpoint != "" {
		cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: conn.Endpoint}, nil
			})
	}

	return &cfg, err
}
