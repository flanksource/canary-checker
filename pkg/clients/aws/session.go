//go:build !fast

package aws

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/henvic/httpretty"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func NewSession(ctx context.Context, conn connection.AWSConnection) (*aws.Config, error) {
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

	loadOptions := []func(*config.LoadOptions) error{
		config.WithHTTPClient(&http.Client{Transport: tr}),
	}
	if conn.Endpoint != "" {
		loadOptions = append(loadOptions,
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: conn.Endpoint}, nil
				}),
			),
		)
	}
	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)

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

	return &cfg, err
}
