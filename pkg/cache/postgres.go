package cache

import (
	gocontext "context"
	"encoding/json"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var PostgresCache = &postgresCache{}

type SummaryOptions query.CheckSummaryOptions

type postgresCache struct {
	context.Context
}

func NewPostgresCache(context context.Context) *postgresCache {
	return &postgresCache{
		Context: context,
	}
}

func (c *postgresCache) Add(check pkg.Check, statii ...pkg.CheckStatus) []string {
	checkIDs := make([]string, 0, len(statii))

	db := c.DB()
	for _, status := range statii {
		if status.Status {
			check.Status = "healthy"
		} else {
			check.Status = "unhealthy"
		}

		if status.Invalid {
			check.Status = "invalid"
		}

		checkID, err := c.AddCheckFromStatus(check, status)
		if err != nil {
			logger.Errorf("error persisting check with canary %s: %v", check.CanaryID, err)
		} else {
			checkIDs = append(checkIDs, checkID.String())
		}
		c.AddCheckStatus(db, check, status)
	}

	return checkIDs
}

func (c *postgresCache) AddCheckFromStatus(check pkg.Check, status pkg.CheckStatus) (uuid.UUID, error) {
	if status.Check == nil {
		return uuid.Nil, nil
	}

	if check.ID != uuid.Nil {
		return check.ID, nil
	}

	return db.PersistCheck(c.DB(), check, check.CanaryID)
}

func (c *postgresCache) AddCheckStatus(db *gorm.DB, check pkg.Check, status pkg.CheckStatus) {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		logger.Errorf("error marshalling details: %v", err)
	}

	checks := pkg.Checks{}
	var nextRuntime *time.Time
	if check.Canary != nil {
		nextRuntime, _ = check.Canary.NextRuntime(time.Now())
	}
	if c.DB().Model(&checks).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		Where("canary_id = ? AND type = ? AND name = ?", check.CanaryID, check.Type, check.GetName()).
		Updates(map[string]any{"status": check.Status, "labels": check.Labels, "last_runtime": time.Now(), "next_runtime": nextRuntime}).Error != nil {
		logger.Errorf("error updating check: %v", err)
		return
	}

	if len(checks) == 0 || checks[0].ID == uuid.Nil {
		logger.Tracef("%s check not found, skipping status insert", check)
		return
	}
	err = db.Exec(`INSERT INTO check_statuses(
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
		VALUES(?,?,?,?,?,?,?,?,NOW())
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
	).Error

	if err != nil {
		logger.Errorf("error adding check status to postgres: %v", err)
	}
}

func (c *postgresCache) GetDetails(checkkey string, time string) interface{} {
	var details interface{}
	row := c.Pool().QueryRow(gocontext.TODO(), `SELECT details from check_statuses where check_id=$1 and time=$2`, checkkey, time)
	if err := row.Scan(&details); err != nil {
		logger.Errorf("error fetching details from check_statuses: %v", err)
	}
	return details
}
