package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
)

var Pool *pgxpool.Pool
var Gorm *gorm.DB
var ConnectionString string
var DefaultExpiryDays int
var RunMigrations bool
var DBMetrics bool
var PostgresServer *embeddedpostgres.EmbeddedPostgres
var HTTPEndpoint = "http://localhost:8080/db"

func Start(ctx context.Context) error {
	if err := Init(); err != nil {
		return err
	}
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
		logger.Infof("Stoped database server")
	}
	return nil
}

func IsConfigured() bool {
	return ConnectionString != "" && ConnectionString != "DB_URL"
}

func IsConnected() bool {
	return Pool != nil
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

func Init() error {
	if ConnectionString == "" || ConnectionString == "DB_URL" {
		logger.Warnf("No db connection string specified")
		return nil
	}
	if Pool != nil {
		return nil
	}

	if strings.HasPrefix(ConnectionString, "embedded://") {
		if err := embeddedDB(); err != nil {
			return err
		}
	}

	var err error
	Pool, err = duty.NewPgxPool(ConnectionString)
	if err != nil {
		return err
	}

	Gorm, err = duty.NewGorm(ConnectionString, duty.DefaultGormConfig())
	if err != nil {
		return err
	}

	if DBMetrics {
		go func() {
			if err := Gorm.Use(prometheus.New(prometheus.Config{
				DBName:      Pool.Config().ConnConfig.Database,
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
		opts := &migrate.MigrateOptions{IgnoreFiles: []string{"007_events.sql", "012_changelog.sql"}}
		if err := duty.Migrate(ConnectionString, opts); err != nil {
			return err
		}
	}

	return nil
}

func GetDB() (*sql.DB, error) {
	return duty.NewDB(ConnectionString)
}
