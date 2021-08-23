package aws

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	v1 "github.com/flanksource/canary-checker/api/v1"
)

func NewSession(ctx *context.Context, conn v1.AWSConnection) (*aws.Config, error) {
	namespace := ctx.Canary.GetNamespace()
	_, accessKey, err := ctx.Kommons.GetEnvValue(conn.AccessKeyID, namespace)
	if err != nil {
		return nil, fmt.Errorf("could not parse EC2 access key: %v", err)
	}
	_, secretKey, err := ctx.Kommons.GetEnvValue(conn.SecretKey, namespace)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Could not parse EC2 secret key: %v", err))
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: conn.SkipTLSVerify},
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion(conn.Region),
		config.WithEndpointResolver(aws.EndpointResolverFunc(
			func(service, region string) (aws.Endpoint, error) {
				return aws.Endpoint{URL: conn.Endpoint}, nil
			})),
		config.WithHTTPClient(&http.Client{Transport: tr}),
	)

	return &cfg, err
}
