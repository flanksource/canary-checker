package cache

import (
	"context"
	"encoding/json"
	"os"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/robfig/cron/v3"
)

var PostgresConnectionString string
var PostgresCacheTimeout int

type postgresCache struct {
	Conn *pgxpool.Pool
}

var PostgresCache = &postgresCache{
	Conn: nil,
}

func InitPostgres(connectionString string) (*pgxpool.Pool, error) {
	var connString string
	// Check if the connectionString Param contains a reference to env
	val := os.Getenv(connectionString)
	if val == "" {
		connString = connectionString
	} else {
		connString = val
	}
	conn, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(context.TODO(), `CREATE TABLE IF NOT EXISTS checks(
		canary json,
		canary_name TEXT,
		check_type TEXT NOT NULL,
		description TEXT,
		display_type TEXT,
		endpoint TEXT,
		icon TEXT,
		id TEXT NOT NULL,
		interval int,
		key TEXT NOT NULL,
		labels json,
		name TEXT NOT NULL,
		namespace TEXT NOT NULL,
		owner TEXT,
		runner_labels json,
		runner_name TEXT,
		schedule TEXT,
		severity TEXT,
		updated_at TIMESTAMP NOT NULL,
		PRIMARY KEY (key)
	)`)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(context.TODO(), `CREATE TABLE IF NOT EXISTS check_statuses(
		check_key TEXT NOT NULL,
		details json,
		duration INT,
		error Text,
		inserted_at TIMESTAMP NOT NULL,
		invalid boolean,
		message TEXT,
		status boolean,
		time TIMESTAMP,
		PRIMARY KEY (time, check_key)
	)`)
	if err != nil {
		return nil, err
	}
	cron := cron.New()
	cron.AddFunc("@every 1d", func() { // nolint: errcheck
		if _, err := conn.Exec(context.TODO(), "DELETE FROM checks WHERE updated_at < NOW() - INTERVAL '1 day' * $1;", PostgresCacheTimeout); err != nil {
			logger.Errorf("error deleting old entried from check")
		}
		if _, err := conn.Exec(context.TODO(), "DELETE FROM check_statuses WHERE inserted_at < NOW() - INTERVAL '1 day' * $1;", PostgresCacheTimeout); err != nil {
			logger.Errorf("error deleting old entried from check")
		}
	})
	cron.Start()
	return conn, nil
}

func (c *postgresCache) Add(check pkg.Check, status pkg.CheckStatus) {
	c.AddCheck(check, status)
	c.AddCheckStatus(check, status)
}

func (c *postgresCache) AddCheck(check pkg.Check, status pkg.CheckStatus) {
	row := c.Conn.QueryRow(context.TODO(), "SELECT key from checks where key=$1", check.Key)
	var key string
	err := row.Scan(&key)
	if err == pgx.ErrNoRows {
		c.InsertCheck(check)
		return
	}
	if err != nil {
		logger.Errorf("error getting check from postgres: %v", err)
		return
	}
	c.UpdateTimestamp()
}

func (c *postgresCache) InsertCheck(check pkg.Check) {
	jsonLabels, err := json.Marshal(check.Labels)
	if err != nil {
		logger.Errorf("Error marshalling labels: %v", err)
	}
	jsonRunnerLabels, err := json.Marshal(check.RunnerLabels)
	if err != nil {
		logger.Errorf("Error marshalling runner labels: %v", err)
	}
	jsonCanary, err := json.Marshal(check.Canary)
	if err != nil {
		logger.Errorf("error marshalling canary: %v", err)
	}
	_, err = c.Conn.Exec(context.TODO(), `INSERT INTO checks(
		canary,
		canary_name,
		check_type,
		description,
		display_type,
		endpoint,
		icon,
		id,
		interval,
		key,
		labels,
		name,
		namespace,
		owner,
		runner_labels,
		runner_name,
		schedule,
		severity,
		updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW())`,
		string(jsonCanary),
		check.CanaryName,
		check.Type,
		check.Description,
		check.DisplayType,
		check.Endpoint,
		check.Icon,
		check.ID,
		check.Interval,
		check.Key,
		string(jsonLabels),
		check.Name,
		check.Namespace,
		check.Owner,
		string(jsonRunnerLabels),
		check.RunnerName,
		check.Schedule,
		check.Severity,
	)
	if err != nil {
		logger.Errorf("error adding check to postgres: %v", err)
	}
}

func (c *postgresCache) UpdateTimestamp() {
	_, err := c.Conn.Exec(context.TODO(), `UPDATE checks SET updated_at=NOW()`)
	if err != nil {
		logger.Errorf("error updating timestamp in checks in postgres: %v", err)
	}
}

func (c *postgresCache) AddCheckStatus(check pkg.Check, status pkg.CheckStatus) {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		logger.Errorf("error marshalling details: %v", err)
	}
	_, err = c.Conn.Exec(context.TODO(), `INSERT INTO check_statuses(
		check_key,
		details,
		duration,
		error,
		inserted_at,
		invalid,
		message,
		status,
		time
		)
		VALUES($1,$2,$3,$4,NOW(),$5,$6,$7,$8)
		`,
		check.Key,
		string(jsonDetails),
		status.Duration,
		status.Error,
		status.Invalid,
		status.Message,
		status.Status,
		status.Time,
	)
	if err != nil {
		logger.Errorf("error adding check status to postgres: %v", err)
	}
}

func (c *postgresCache) GetChecks() pkg.Checks {
	var result pkg.Checks
	rows, err := c.Conn.Query(context.TODO(), `SELECT key FROM checks order by key`)
	if err != nil {
		logger.Errorf("error getting checks from postgres: %v", err)
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			logger.Errorf("error scanning check row: %v", err)
			continue
		}
		check := c.GetCheckFromKey(key)
		result = append(result, check)
	}
	return result
}

func (c *postgresCache) GetCheckFromKey(key string) *pkg.Check {
	row := c.Conn.QueryRow(context.TODO(), "SELECT * FROM checks WHERE key=$1", key)
	var checkType, name, namespace, canaryName, description, endpoint, schedule, severity, owner, icon, displayType, runnerName, id string
	var canary *v1.Canary
	var labels, runnerLabels map[string]string
	var checkUpdatedAt time.Time
	var interval int
	if err := row.Scan(&canary, &canaryName, &checkType, &description, &displayType, &endpoint, &icon, &id, &interval, &key, &labels, &name, &namespace, &owner, &runnerLabels, &runnerName, &schedule, &severity, &checkUpdatedAt); err != nil {
		logger.Errorf("error scanning check row: %v", err)
		return nil
	}
	var passed, failed int
	var latencyR1H pgtype.Float4
	passRow := c.Conn.QueryRow(context.TODO(), `SELECT COUNT(1) as passed from check_statuses where status=true and check_key=$1 and (inserted_at > NOW() - Interval '1 hour')`, key)
	if err := passRow.Scan(&passed); err != nil {
		logger.Errorf("error scanning check status row for pass statuses: %v", err)
	}
	failRow := c.Conn.QueryRow(context.TODO(), `SELECT COUNT(1) as passed from check_statuses where status!=true and check_key=$1 and (inserted_at > NOW() - Interval '1 hour')`, key)
	if err := failRow.Scan(&failed); err != nil {
		logger.Errorf("error scanning check status row for fail statuses: %v", err)
	}
	latencyRow := c.Conn.QueryRow(context.TODO(), `SELECT percentile_disc(0.99) within group (order by check_statuses.duration) from check_statuses where check_key=$1 and (inserted_at > NOW() - Interval '1 hour')`, key)
	if err := latencyRow.Scan(&latencyR1H); err != nil {
		logger.Errorf("error scanning check status row for latency: %v", err)
	}
	check := &pkg.Check{
		Key:          key,
		Type:         checkType,
		Name:         name,
		Namespace:    namespace,
		Labels:       labels,
		RunnerLabels: runnerLabels,
		CanaryName:   canaryName,
		Description:  description,
		Endpoint:     endpoint,
		Uptime: pkg.Uptime{
			Passed: passed,
			Failed: failed,
		},
		Latency: pkg.Latency{
			Rolling1H: float64(latencyR1H.Float),
		},
		Interval:    uint64(interval),
		Schedule:    schedule,
		Owner:       owner,
		Severity:    severity,
		Icon:        icon,
		DisplayType: displayType,
		RunnerName:  runnerName,
		ID:          id,
		Canary:      canary,
	}
	return check
}

func (c *postgresCache) RemoveChecks(canary v1.Canary) {
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		logger.Debugf("removing %s", key)
		c.RemoveCheckByKey(key)
	}
}

func (c *postgresCache) RemoveCheckByKey(key string) {
	if _, err := c.Conn.Exec(context.TODO(), `DELETE FROM checks WHERE key=$1`, key); err != nil {
		logger.Errorf("error deleting check from postgres: %v", err)
	}
	if _, err := c.Conn.Exec(context.TODO(), `DELETE FROM check_statuses WHERE check_key=$1`, key); err != nil {
		logger.Errorf("error deleting check_statuses from postgres: %v", err)
	}
}

func (c *postgresCache) ListCheckStatus(checkKey string, count int64, duration *time.Duration) []pkg.CheckStatus {
	if duration != nil {
		if count == AllStatuses {
			return c.ListAllCheckStatus(checkKey, duration)
		}
		statusRows, err := c.Conn.Query(context.TODO(), `SELECT * FROM check_statuses WHERE check_key=$1 and (inserted_at > NOW() - Interval '1 SECOND' * $2) ORDER BY inserted_at DESC LIMIT $3`, checkKey, duration.Seconds(), count)
		if err != nil {
			logger.Errorf("error querying check_statuses: %v", err)
			return nil
		}
		return scanStatusRows(statusRows)
	}
	statusRows, err := c.Conn.Query(context.TODO(), `SELECT * FROM check_statuses WHERE check_key=$1 ORDER BY inserted_at DESC LIMIT $2`, checkKey, count)
	if err != nil {
		logger.Errorf("error querying check_statuses: %v", err)
		return nil
	}
	return scanStatusRows(statusRows)
}

func (c *postgresCache) ListAllCheckStatus(checkKey string, duration *time.Duration) []pkg.CheckStatus {
	statusRows, err := c.Conn.Query(context.TODO(), `SELECT * FROM check_statuses WHERE check_key=$1 and (inserted_at > NOW() - Interval '1 SECOND' * $2) ORDER BY inserted_at DESC`, checkKey, duration.Seconds())
	if err != nil {
		logger.Errorf("error querying check_statuses: %v", err)
		return nil
	}
	return scanStatusRows(statusRows)
}

func scanStatusRows(statusRows pgx.Rows) []pkg.CheckStatus {
	var result []pkg.CheckStatus
	for statusRows.Next() {
		var status, invalid bool
		var message, error, checkKey string
		var duration int
		var details interface{}
		var statusTime, statusInsertedAt time.Time
		if err := statusRows.Scan(&checkKey, &details, &duration, &error, &statusInsertedAt, &invalid, &message, &status, &statusTime); err != nil {
			logger.Errorf("error scanning check status row: %v", err)
			continue
		}
		result = append(result, pkg.CheckStatus{
			Status:   status,
			Invalid:  invalid,
			Time:     statusTime.UTC().Format(time.RFC3339),
			Duration: duration,
			Message:  message,
			Error:    error,
			Detail:   details,
		})
	}
	return result
}

func (c *postgresCache) GetDetails(checkkey string, time string) interface{} {
	var details interface{}
	row := c.Conn.QueryRow(context.TODO(), `SELECT details from check_statuses where check_key=$1 and time=$2`, checkkey, time)
	if err := row.Scan(&details); err != nil {
		logger.Errorf("error fetching details from check_statuses: %v", err)
	}
	return details
}
