package db

import (
	"context"
	"database/sql"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	dutyContext "github.com/flanksource/duty/context"
)

var defaultContext *dutyContext.Context
var ConnectionString string
var DefaultExpiryDays int
var RunMigrations bool
var DBMetrics bool
var PostgresServer *embeddedpostgres.EmbeddedPostgres
var HTTPEndpoint = "http://localhost:8080/db"

func Start(ctx context.Context) error {
	<-ctx.Done()
	return StopServer()
}

func StopServer() error {
	if PostgresServer != nil {
		logger.Infof("Stopping database server")
		err := PostgresServer.Stop()
		if err != nil {
			return err
		}
		PostgresServer = nil
		logger.Infof("Stopped database server")
	}
	return nil
}

func IsConfigured() bool {
	return ConnectionString != "" && ConnectionString != "DB_URL"
}

func IsConnected() bool {
	return defaultContext != nil
}

func GetDB() (*sql.DB, error) {
	return duty.NewDB(ConnectionString)
}
