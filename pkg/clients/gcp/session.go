package gcp

import (
	gcs "cloud.google.com/go/storage"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"google.golang.org/api/option"
)

func NewSession(ctx *context.Context, conn *v1.GCPConnection) (*gcs.Client, error) {
	conn = conn.Validate()
	var client *gcs.Client
	var err error
	if conn.Credentials != nil {
		_, credential, err := ctx.Kommons.GetEnvValue(*conn.Credentials, ctx.Canary.GetNamespace())
		if err != nil {
			return nil, err
		}
		client, err = gcs.NewClient(ctx.Context, option.WithEndpoint(conn.Endpoint), option.WithCredentialsJSON([]byte(credential)))
		if err != nil {
			return nil, err
		}
	} else {
		client, err = gcs.NewClient(ctx.Context, option.WithEndpoint(conn.Endpoint))
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}
