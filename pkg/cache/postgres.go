package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm/clause"
)

var PostgresCache = &postgresCache{}

type postgresCache struct {
	*pgxpool.Pool
}

func NewPostgresCache(pool *pgxpool.Pool) *postgresCache {
	return &postgresCache{
		Pool: pool,
	}
}

func (c *postgresCache) Add(check pkg.Check, statii ...pkg.CheckStatus) {
	for _, status := range statii {
		if status.Status {
			check.Status = "healthy"
		} else {
			check.Status = "unhealthy"
		}
		c.AddCheckStatus(check, status)
	}
}

func (c *postgresCache) AddCheckStatus(check pkg.Check, status pkg.CheckStatus) {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		logger.Errorf("error marshalling details: %v", err)
	}
	checks := pkg.Checks{}
	if db.Gorm.Model(&checks).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		Where("canary_id = ? AND type = ? AND name = ?", check.CanaryID, check.Type, check.GetName()).
		Updates(map[string]any{"status": check.Status, "labels": check.Labels, "last_runtime": time.Now()}).Error != nil {
		logger.Errorf("error updating check: %v", err)
		return
	}
	if len(checks) == 0 || checks[0].ID == uuid.Nil {
		logger.Debugf("check not found")
		return
	}
	_, err = c.Exec(context.TODO(), `INSERT INTO check_statuses(
		check_id,
		details,
		duration,
		error,
		invalid,
		message,
		status,
		time,
		created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (check_id,time) DO NOTHING;
		`,
		checks[0].ID,
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

func (c *postgresCache) Query(q QueryParams) (pkg.Checks, error) {
	return q.ExecuteSummary(db.Pool)
}

func (c *postgresCache) QuerySummary() (pkg.Checks, error) {
	query := `
SELECT
    checks.id::text,
    checks.canary_id::text,
    check_status_summary.passed,
    check_status_summary.failed,
    check_status_summary.p99,
    check_status_summary.last_pass,
    check_status_summary.last_fail,
    checs.last_transition_time,
    checks.type,
    checks.icon,
    checks.name,
    checks.description,
    canaries.namespace,
    canaries.name as canaryName,
    canaries.labels,
    checks.severity,
    checks.owner,
    checks.last_runtime,
    checks.created_at,
    checks.updated_at,
    checks.deleted_at,
    checks.silenced_at
FROM checks

INNER JOIN canaries on checks.canary_id = canaries.id

INNER JOIN check_status_summary ON checks.id = check_status_summary.check_id
`
	rows, err := db.Pool.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	var checks pkg.Checks
	for rows.Next() {
		var check pkg.Check
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		check.ID, _ = uuid.Parse(vals[0].(string))
		check.CanaryID, _ = uuid.Parse(vals[1].(string))
		check.Uptime.Passed = intV(vals[2])
		check.Uptime.Failed = intV(vals[3])
		check.Latency.Percentile99 = float64V(vals[4])
		check.Uptime.LastPass, _ = timeV(vals[5])
		check.Uptime.LastFail, _ = timeV(vals[6])
		check.LastTransitionTime, _ = timeV(vals[7])
		check.Type = vals[8].(string)
		check.Icon = vals[9].(string)
		check.Name = vals[10].(string)
		check.Description = vals[11].(string)
		check.Namespace = vals[12].(string)
		check.CanaryName = vals[13].(string)
		check.Labels = mapStringString(vals[14])
		check.Severity = vals[15].(string)
		check.Owner = vals[16].(string)
		check.LastRuntime, _ = timeV(vals[17])
		check.CreatedAt, _ = timeV(vals[18])
		check.UpdatedAt, _ = timeV(vals[19])
		check.SilencedAt, _ = timeV(vals[20])

		checks = append(checks, &check)
	}
	return checks, nil
}

func (c *postgresCache) QueryStatus(q QueryParams) ([]pkg.Timeseries, error) {
	return q.ExecuteDetails(db.Pool)
}

func (c *postgresCache) GetDetails(checkkey string, time string) interface{} {
	var details interface{}
	row := c.QueryRow(context.TODO(), `SELECT details from check_statuses where check_id=$1 and time=$2`, checkkey, time)
	if err := row.Scan(&details); err != nil {
		logger.Errorf("error fetching details from check_statuses: %v", err)
	}
	return details
}
