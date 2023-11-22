package gcp

import (
	gcs "cloud.google.com/go/storage"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"google.golang.org/api/option"
)

func NewSession(ctx context.Context, conn *connection.GCPConnection) (*gcs.Client, error) {
	conn = conn.Validate()
	var client *gcs.Client
	var err error
	if !conn.Credentials.IsEmpty() {
		credential, err := ctx.GetEnvValueFromCache(*conn.Credentials)
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
