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
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
)

var Pool *pgxpool.Pool
var Gorm *gorm.DB
var ConnectionString string
var DefaultExpiryDays int
var RunMigrations bool
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
	err := os.Chmod(embeddedPath, 0750)
	if err != nil {
		logger.Errorf("Error changing permission of dataPath: %v, Error: %v", embeddedPath, err)
		return err
	}

	logger.Infof("Starting embedded postgres server at %s", embeddedPath)

	PostgresServer = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(6432).
		DataPath(path.Join(embeddedPath, "data")).
		RuntimePath(embeddedPath).
		BinariesPath(embeddedPath).
		Version(embeddedpostgres.V14).
		Username("postgres").Password("postgres").
		Database("canary"))
	ConnectionString = "postgres://postgres:postgres@localhost:6432/canary?sslmode=disable"
	err = PostgresServer.Start()
	if err != nil {
		return fmt.Errorf("error starting embedded postgres: %v", err)
	}
	return nil
}

func Init() error {
	if ConnectionString == "" {
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

	if err := Gorm.Use(prometheus.New(prometheus.Config{
		DBName:      Pool.Config().ConnConfig.Database,
		StartServer: false,
		MetricsCollector: []prometheus.MetricsCollector{
			&prometheus.Postgres{},
		},
	})); err != nil {
		logger.Warnf("Failed to register prometheus metrics: %v", err)
	}

	if RunMigrations {
		if err := duty.Migrate(ConnectionString); err != nil {
			return err
		}
	}

	return nil
}

func Cleanup() {
	cron := cron.New()
	cron.AddFunc("@every 1d", func() { // nolint: errcheck
		if _, err := Pool.Exec(context.TODO(), "DELETE FROM checks WHERE updated_at < NOW() - INTERVAL '1 day' * $1;", DefaultExpiryDays); err != nil {
			logger.Errorf("error deleting old entried from check")
		}
		if _, err := Pool.Exec(context.TODO(), "DELETE FROM check_statuses WHERE inserted_at < NOW() - INTERVAL '1 day' * $1;", DefaultExpiryDays); err != nil {
			logger.Errorf("error deleting old entried from check")
		}
	})
	cron.Start()
}

func GetDB() (*sql.DB, error) {
	return duty.NewDB(ConnectionString)
}

func ConvertNamedParams(sql string, namedArgs map[string]interface{}) (string, []interface{}) {
	i := 1
	var args []interface{}
	// Loop the named args and replace with placeholders
	for pname, pval := range namedArgs {
		sql = strings.ReplaceAll(sql, ":"+pname, fmt.Sprint(`$`, i))
		args = append(args, pval)
		i++
	}
	return sql, args
}

func QueryNamed(ctx context.Context, sql string, args map[string]interface{}) (pgx.Rows, error) {
	sql, namedArgs := ConvertNamedParams(sql, args)
	return Pool.Query(ctx, sql, namedArgs...)
}
