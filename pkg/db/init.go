package db

import (
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/api"
	dutyContext "github.com/flanksource/duty/context"
)

var defaultContext *dutyContext.Context
var DefaultExpiryDays int
var HTTPEndpoint = "http://localhost:8080/db"

func IsConfigured() bool {
	return api.DefaultConfig.ConnectionString != "" && api.DefaultConfig.ConnectionString != "DB_URL"
}

func IsConnected() bool {
	return defaultContext != nil
}

func Init() (dutyContext.Context, error) {
	ctx, stopper, err := duty.Start("canary-checker", duty.SkipMigrationByDefaultMode, duty.SkipChangelogMigration)
	if err != nil {
		return dutyContext.New(), err
	}
	runner.AddShutdownHook(stopper)

	return ctx, nil
}
