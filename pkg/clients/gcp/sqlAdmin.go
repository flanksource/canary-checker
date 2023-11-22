package gcp

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/duty/connection"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

func NewSQLAdmin(ctx *context.Context, conn *connection.GCPConnection) (*sqladmin.Service, error) {
	conn = conn.Validate()
	var err error
	var client *sqladmin.Service
	if !conn.Credentials.IsEmpty() {
		credential, err := ctx.GetEnvValueFromCache(*conn.Credentials)
		if err != nil {
			return nil, err
		}
		client, err = sqladmin.NewService(ctx.Context, option.WithEndpoint(conn.Endpoint), option.WithCredentialsJSON([]byte(credential)))
		if err != nil {
			return nil, err
		}
	} else {
		client, err = sqladmin.NewService(ctx.Context, option.WithEndpoint(conn.Endpoint))
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}
