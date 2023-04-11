package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/models"
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
		c.AddCheckFromStatus(check, status)
		c.AddCheckStatus(check, status)
	}
}

func (c *postgresCache) AddCheckFromStatus(check pkg.Check, status pkg.CheckStatus) {
	if status.Check == nil {
		return
	}
	// Before syncing canary, mark all these checks as deleted_at
	if _, err := db.PersistCheck(check, check.CanaryID); err != nil {
		logger.Errorf("error persisting check with canary %s: %v", check.CanaryID, err)
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

func (c *postgresCache) QuerySummary() (models.Checks, error) {
	return duty.QueryCheckSummary(db.Pool)
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
