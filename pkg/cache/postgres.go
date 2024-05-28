package cache

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/query"
	"github.com/google/uuid"
	"github.com/samber/lo"
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

func (c *postgresCache) saveCheckAndStatus(check pkg.Check, status pkg.CheckStatus) (string, error) {
	tx := c.DB().Begin()
	if tx.Error != nil {
		return "", fmt.Errorf("error starting transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	check.Status = lo.Ternary(status.Status, "healthy", "unhealthy")
	checkID, err := AddCheckFromStatus(tx, check, status)
	if err != nil {
		return "", fmt.Errorf("error persisting check with canary %s: %w", check.CanaryID, err)
	}

	if err := c.AddCheckStatus(tx, check, status); err != nil {
		return "", fmt.Errorf("error persisting check status with canary %s: %w", check.CanaryID, err)
	}

	return checkID.String(), tx.Commit().Error
}

func (c *postgresCache) Add(check pkg.Check, statii ...pkg.CheckStatus) []string {
	checkIDs := make([]string, 0, len(statii))
	for _, status := range statii {
		if checkID, err := c.saveCheckAndStatus(check, status); err != nil {
			logger.Errorf("error saving check and status: %v", err)
		} else {
			checkIDs = append(checkIDs, checkID)
		}
	}

	return checkIDs
}

func AddCheckFromStatus(tx *gorm.DB, check pkg.Check, status pkg.CheckStatus) (uuid.UUID, error) {
	if status.Check == nil {
		return uuid.Nil, nil
	}

	if check.ID != uuid.Nil {
		return check.ID, nil
	}

	return db.PersistCheck(tx, check, check.CanaryID)
}

func (c *postgresCache) AddCheckStatus(conn *gorm.DB, check pkg.Check, status pkg.CheckStatus) error {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		return fmt.Errorf("error marshalling details: %w", err)
	}

	checks := pkg.Checks{}
	var nextRuntime *time.Time
	if check.Canary != nil {
		nextRuntime, _ = check.Canary.NextRuntime(time.Now())
	}

	if conn.Model(&checks).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		Where("canary_id = ? AND type = ? AND name = ?", check.CanaryID, check.Type, check.GetName()).
		Updates(map[string]any{"status": check.Status, "labels": check.Labels, "last_runtime": status.Time, "next_runtime": nextRuntime}).Error != nil {
		return fmt.Errorf("error updating check: %w", err)
	}

	if len(checks) == 0 || checks[0].ID == uuid.Nil {
		logger.Tracef("%s check not found, skipping status insert", check)
		return nil
	}

	err = conn.Exec(`INSERT INTO check_statuses(
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
		return fmt.Errorf("error adding check status to postgres: %w", err)
	}

	return nil
}

func (c *postgresCache) GetDetails(checkkey string, time string) interface{} {
	var details interface{}
	row := c.Pool().QueryRow(gocontext.TODO(), `SELECT details from check_statuses where check_id=$1 and time=$2`, checkkey, time)
	if err := row.Scan(&details); err != nil {
		logger.Errorf("error fetching details from check_statuses: %v", err)
	}
	return details
}
