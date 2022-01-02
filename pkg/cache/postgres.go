package cache

import (
	"context"
	"encoding/json"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type postgresCache struct {
	*pgxpool.Pool
}

func NewPostgresCache(pool *pgxpool.Pool) *postgresCache {
	return &postgresCache{
		Pool: pool,
	}
}

func (c *postgresCache) Add(check pkg.Check, statii ...pkg.CheckStatus) {
	c.AddCheck(check)
	for _, status := range statii {
		c.AddCheckStatus(check, status)
	}
}

func (c *postgresCache) AddCheck(check pkg.Check) {
	row := c.QueryRow(context.TODO(), "SELECT key from checks where key=$1", check.Key)
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
	c.UpdateTimestamp(check)
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
	_, err = c.Exec(context.TODO(), `INSERT INTO checks(
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

func (c *postgresCache) UpdateTimestamp(check pkg.Check) {
	_, err := c.Exec(context.TODO(), `UPDATE checks SET updated_at=NOW() WHERE key = $1`, check.Key)
	if err != nil {
		logger.Errorf("error updating timestamp in checks in postgres: %v", err)
	}
}

func (c *postgresCache) AddCheckStatus(check pkg.Check, status pkg.CheckStatus) {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		logger.Errorf("error marshalling details: %v", err)
	}
	_, err = c.Exec(context.TODO(), `INSERT INTO check_statuses(
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
		ON CONFLICT (check_key,time) DO NOTHING;
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

func (c *postgresCache) RemoveChecks(canary v1.Canary) {
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		logger.Debugf("removing %s", key)
		c.RemoveCheckByKey(key)
	}
}

func (c *postgresCache) RemoveCheckByKey(key string) {
	if _, err := c.Exec(context.TODO(), `DELETE FROM checks WHERE key=$1`, key); err != nil {
		logger.Errorf("error deleting check from postgres: %v", err)
	}
	if _, err := c.Exec(context.TODO(), `DELETE FROM check_statuses WHERE check_key=$1`, key); err != nil {
		logger.Errorf("error deleting check_statuses from postgres: %v", err)
	}
}

func (c *postgresCache) Query(q QueryParams) (pkg.Checks, error) {
	return q.ExecuteSummary(db.Pool)
}

func (c *postgresCache) QueryStatus(q QueryParams) ([]pkg.Timeseries, error) {
	return q.ExecuteDetails(db.Pool)
}

func (c *postgresCache) GetDetails(checkkey string, time string) interface{} {
	var details interface{}
	row := c.QueryRow(context.TODO(), `SELECT details from check_statuses where check_key=$1 and time=$2`, checkkey, time)
	if err := row.Scan(&details); err != nil {
		logger.Errorf("error fetching details from check_statuses: %v", err)
	}
	return details
}
