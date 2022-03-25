package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/commons/logger"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/log/logrusadapter"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

var Pool *pgxpool.Pool
var Gorm *gorm.DB
var ConnectionString string
var DefaultExpiryDays int
var pgxConnectionString string
var PostgresServer *embeddedpostgres.EmbeddedPostgres
var Trace bool

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

func Init() error {
	if ConnectionString == "" {
		logger.Warnf("No db connection string specified")
		return nil
	}
	if Pool != nil {
		return nil
	}
	if strings.HasPrefix(ConnectionString, "embedded://") {
		runtimePath := strings.ReplaceAll(ConnectionString, "embedded://", "")
		logger.Infof("Starting embedded postgres server at %s", runtimePath)
		PostgresServer = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
			Port(6432).
			DataPath(runtimePath).
			BinariesPath(".bin/postgres").
			Version(embeddedpostgres.V14).
			Username("postgres").Password("postgres").
			Database("canary"))
		ConnectionString = "postgres://postgres:postgres@localhost:6432/canary"
		err := PostgresServer.Start()
		if err != nil {
			return err
		}
	}

	config, err := pgxpool.ParseConfig(ConnectionString)
	if err != nil {
		if err != nil {
			return err
		}
	}

	if Trace {
		logrusLogger := &logrus.Logger{
			Out:          os.Stderr,
			Formatter:    new(logrus.TextFormatter),
			Hooks:        make(logrus.LevelHooks),
			Level:        logrus.DebugLevel,
			ExitFunc:     os.Exit,
			ReportCaller: false,
		}
		config.ConnConfig.Logger = logrusadapter.NewLogger(logrusLogger)
		boil.DebugMode = true
	}
	Pool, err = pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return err
	}

	row := Pool.QueryRow(context.TODO(), "SELECT pg_size_pretty(pg_database_size($1));", config.ConnConfig.Database)
	var size string
	if err := row.Scan(&size); err != nil {
		Pool = nil
		return err
	}
	logger.Infof("Initialized DB: %s (%s)", config.ConnString(), size)
	pgxConnectionString = stdlib.RegisterConnConfig(config.ConnConfig)

	db, err := GetDB()
	if err != nil {
		return err
	}
	boil.SetDB(db)

	Gorm, err = gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		FullSaveAssociations: true,
	})

	if logger.IsTraceEnabled() {
		Gorm.Logger.LogMode(glogger.Info)
	}
	if err != nil {
		return err
	}

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
