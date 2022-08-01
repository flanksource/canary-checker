package cache

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
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
		check.Status = status.Status
		c.AddCheckStatus(check, status)
	}
}

func (c *postgresCache) AddCheckStatus(check pkg.Check, status pkg.CheckStatus) {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		logger.Errorf("error marshalling details: %v", err)
	}
	row := c.QueryRow(context.TODO(), `UPDATE checks SET 
										last_runtime = NOW(), 
										status = $1 
										WHERE canary_id = $2 AND type = $3 AND name = $4 
										RETURNING id`,
		status.Status, check.CanaryID, check.Type, check.GetName())
	var id string

	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			check.Status = status.Status
			if id, err = db.PersistCheck(check); err != nil {
				logger.Errorf("error inserting check: %v", err)
				return
			}
		} else {
			logger.Errorf("error fetching check id: %v", err)
			return
		}
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
		id,
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
