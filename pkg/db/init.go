package db

import (
	"context"
	"database/sql"
	"embed"
	"os"
	"strings"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/commons/logger"
	"github.com/jackc/pgx/v4/log/logrusadapter"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

var Pool *pgxpool.Pool
var ConnectionString string
var DefaultExpiryDays int
var pgxConnectionString string
var PostgresServer *embeddedpostgres.EmbeddedPostgres

func StopServer() error {
	if PostgresServer != nil {
		logger.Infof("Stopping database server")
		err := PostgresServer.Stop()
		if err != nil {
			return err
		}
		PostgresServer = nil
	}
	return nil
}

func Init(connection string) error {
	var connString string
	// Check if the connectionString Param contains a reference to env
	val := os.Getenv(connection)
	if val == "" {
		connString = connection
	} else {
		connString = val
	}

	if strings.HasPrefix(connString, "embedded://") {
		runtimePath := strings.ReplaceAll(connString, "embedded://", "")
		PostgresServer = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
			RuntimePath(runtimePath).
			Database("canarychecker"))
		connString = "postgres://postgres:postgres@localhost/canarychecker"
		err := PostgresServer.Start()
		if err != nil {
			return err
		}
	}

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		if err != nil {
			return err
		}
	}

	if logger.IsTraceEnabled() {
		logrusLogger := &logrus.Logger{
			Out:          os.Stderr,
			Formatter:    new(logrus.TextFormatter),
			Hooks:        make(logrus.LevelHooks),
			Level:        logrus.DebugLevel,
			ExitFunc:     os.Exit,
			ReportCaller: false,
		}
		config.ConnConfig.Logger = logrusadapter.NewLogger(logrusLogger)
	}
	Pool, err = pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return err
	}

	row := Pool.QueryRow(context.TODO(), "SELECT pg_size_pretty(pg_database_size($1));", config.ConnConfig.Database)
	var size string
	if err := row.Scan(&size); err != nil {
		return err
	}
	logger.Infof("Initialized DB: %s (%s)", config.ConnString(), size)

	pgxConnectionString = stdlib.RegisterConnConfig(config.ConnConfig)

	return Migrate()
}

func Migrate() error {
	goose.SetBaseFS(embedMigrations)
	db, err := GetDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Up(db, "migrations"); err != nil {
		return err
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
	return sql.Open("pgx", pgxConnectionString)
}
