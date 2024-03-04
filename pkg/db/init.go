package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/migrate"
	"github.com/flanksource/duty/models"
	"github.com/samber/lo"
	"gorm.io/plugin/prometheus"
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

func embeddedDB() error {
	embeddedPath := strings.TrimSuffix(strings.TrimPrefix(ConnectionString, "embedded://"), "/")
	_ = os.Chmod(embeddedPath, 0750)

	logger.Infof("Starting embedded postgres server at %s", embeddedPath)

	PostgresServer = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(6432).
		DataPath(path.Join(embeddedPath, "data")).
		RuntimePath(path.Join(embeddedPath, "runtime")).
		BinariesPath(path.Join(embeddedPath, "bin")).
		Version(embeddedpostgres.V14).
		Username("postgres").Password("postgres").
		Database("canary"))
	ConnectionString = "postgres://postgres:postgres@localhost:6432/canary?sslmode=disable"
	if err := PostgresServer.Start(); err != nil {
		return fmt.Errorf("error starting embedded postgres: %v", err)
	}
	return nil
}

func Connect() (*dutyContext.Context, error) {
	if ConnectionString == "" || ConnectionString == "DB_URL" {
		logger.Warnf("No db connection string specified")
		return nil, nil
	}
	if defaultContext != nil {
		return defaultContext, nil
	}

	if strings.HasPrefix(ConnectionString, "embedded://") {
		if err := embeddedDB(); err != nil {
			return nil, err
		}
	}

	var err error
	Pool, err := duty.NewPgxPool(ConnectionString)
	if err != nil {
		return nil, err
	}

	Gorm, err := duty.NewGorm(ConnectionString, duty.DefaultGormConfig())
	if err != nil {
		return nil, err
	}
	ctx := dutyContext.New().WithDB(Gorm, Pool)
	defaultContext = &ctx
	return defaultContext, nil
}

func Init() (dutyContext.Context, error) {
	if defaultContext != nil {
		return *defaultContext, nil
	}
	var ctx *dutyContext.Context
	var err error
	if ctx, err = Connect(); err != nil {
		return dutyContext.New(), err
	}
	if ctx == nil {
		return dutyContext.New(), nil
	}

	if DBMetrics {
		go func() {
			if err := ctx.DB().Use(prometheus.New(prometheus.Config{
				DBName:      ctx.Pool().Config().ConnConfig.Database,
				StartServer: false,
				MetricsCollector: []prometheus.MetricsCollector{
					&prometheus.Postgres{},
				},
			})); err != nil {
				logger.Warnf("Failed to register prometheus metrics: %v", err)
			}
		}()
	}

	if RunMigrations {
		opts := &migrate.MigrateOptions{IgnoreFiles: []string{"007_events.sql", "012_changelog_triggers_others.sql", "012_changelog_triggers_scrapers.sql"}}
		if err := duty.Migrate(ConnectionString, opts); err != nil {
			return dutyContext.New(), err
		}
	} else {
		_, _, err = lo.AttemptWithDelay(5, 5*time.Second, func(i int, d time.Duration) error {
			err := ctx.DB().Limit(1).Find(&[]models.Agent{}).Error
			if err != nil && strings.Contains(err.Error(), "ERROR: relation \"agents\"") {
				runner.ShutdownAndExit(1, "database migrations not run, use --db-migrations")
			}
			return err
		})
		if err != nil {
			runner.ShutdownAndExit(1, err.Error())
		}
	}
	return *ctx, nil
}

func GetDB() (*sql.DB, error) {
	return duty.NewDB(ConnectionString)
}
