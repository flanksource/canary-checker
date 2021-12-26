// +build aws

package aws

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/kommons"
	"github.com/henvic/httpretty"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	v1 "github.com/flanksource/canary-checker/api/v1"
)

func isEmpty(val kommons.EnvVar) bool {
	return val.Value == "" && val.ValueFrom == nil
}

func NewSession(ctx *context.Context, conn v1.AWSConnection) (*aws.Config, error) {
	namespace := ctx.Canary.GetNamespace()
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

	if !isEmpty(conn.AccessKey) {
		_, accessKey, err := ctx.Kommons.GetEnvValue(conn.AccessKey, namespace)
		if err != nil {
			return nil, fmt.Errorf("could not parse EC2 access key: %v", err)
		}
		_, secretKey, err := ctx.Kommons.GetEnvValue(conn.SecretKey, namespace)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Could not parse EC2 secret key: %v", err))
		}

		cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	}
	if conn.Region != "" {
		cfg.Region = conn.Region
	}
	if conn.Endpoint != "" {
		cfg.EndpointResolver = aws.EndpointResolverFunc(
			func(service, region string) (aws.Endpoint, error) {
				return aws.Endpoint{URL: conn.Endpoint}, nil
			})
	}

	return &cfg, err
}
