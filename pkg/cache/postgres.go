package cache

import (
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	cachelib "github.com/eko/gocache/lib/v4/cache"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/cache"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

var PostgresCache = &postgresCache{}

type postgresCache struct {
	context.Context
	checkStatusCache cachelib.CacheInterface[models.CheckHealthStatus]
	checkIDCache     cachelib.CacheInterface[uuid.UUID]
}

func NewPostgresCache(context context.Context) *postgresCache {
	return &postgresCache{
		Context:          context,
		checkStatusCache: cache.NewCache[models.CheckHealthStatus]("check_status", 2*time.Hour),
		checkIDCache:     cache.NewCache[uuid.UUID]("check_id", 2*time.Hour),
	}
}

func (c *postgresCache) Add(ctx context.Context, check pkg.Check, status pkg.CheckStatus) (string, error) {
	check.Status = lo.Ternary[models.CheckHealthStatus](status.Status, models.CheckStatusHealthy, models.CheckStatusUnhealthy)
	checkID, err := AddCheckFromStatus(ctx, check, status)
	if err != nil {
		return "", fmt.Errorf("error persisting check with canary %s: %w", check.CanaryID, err)
	}

	if err := c.AddCheckStatus(ctx, check, status); err != nil {
		return "", fmt.Errorf("error persisting check status with canary %s: %w", check.CanaryID, err)
	}

	return checkID.String(), nil
}

func AddCheckFromStatus(ctx context.Context, check pkg.Check, status pkg.CheckStatus) (uuid.UUID, error) {
	if status.Check == nil {
		return uuid.Nil, nil
	}

	if check.ID != uuid.Nil && !check.Transformed {
		return check.ID, nil
	}

	return db.PersistCheck(ctx.DB(), check, check.CanaryID)
}

func (c *postgresCache) AddCheckStatus(ctx context.Context, check pkg.Check, status pkg.CheckStatus) error {
	jsonDetails, err := json.Marshal(status.Detail)
	if err != nil {
		return fmt.Errorf("error marshalling details: %w", err)
	}

	var nextRuntime *time.Time
	if check.Canary != nil {
		nextRuntime, _ = check.Canary.NextRuntime(time.Now())
	}

	if check.ID == uuid.Nil {
		checkID, err := c.GetCheckID(ctx, check.CanaryID, check.Type, check.GetName())
		if err != nil {
			return fmt.Errorf("check not found: %w", err)
		}
		check.ID = checkID
	}

	statusInCache, _ := c.checkStatusCache.Get(ctx, check.ID)
	if statusInCache == "" || statusInCache != check.Status {
		q := ctx.DB().Model(&models.Check{}).
			Where("id = ?", check.ID).
			Where("status != ?", check.Status)

		if err := q.UpdateColumn("status", check.Status).Error; err != nil {
			return fmt.Errorf("error updating check: %w", err)
		}
		_ = c.checkStatusCache.Set(ctx, check.ID, check.Status)
	}

	t, _ := status.GetTime()
	up := models.ChecksUnlogged{
		CheckID:     check.ID,
		CanaryID:    check.CanaryID,
		Status:      string(check.Status),
		LastRuntime: lo.ToPtr(t),
		NextRuntime: nextRuntime,
	}
	if err := ctx.DB().Save(&up).Error; err != nil {
		return fmt.Errorf("error updating checks_unlogged: %w", err)
	}

	err = ctx.DB().Exec(`INSERT INTO check_statuses(
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
		check.ID,
		string(jsonDetails),
		status.DurationMs,
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

func (c *postgresCache) GetDetails(checkkey string, time string) any {
	var details any
	row := c.Pool().QueryRow(gocontext.TODO(), `SELECT details from check_statuses where check_id=$1 and time=$2`, checkkey, time)
	if err := row.Scan(&details); err != nil {
		logger.Errorf("error fetching details from check_statuses: %v", err)
	}
	return details
}

func (c postgresCache) generateCheckIDCacheKey(canaryID uuid.UUID, checkType, checkName string) string {
	return strings.Join([]string{canaryID.String(), checkType, checkName}, ".")
}

func (c *postgresCache) GetCheckID(ctx context.Context, canaryID uuid.UUID, checkType, checkName string) (uuid.UUID, error) {
	cacheKey := c.generateCheckIDCacheKey(canaryID, checkType, checkName)
	if val, _ := c.checkIDCache.Get(ctx, cacheKey); val != uuid.Nil {
		return val, nil
	}

	var check models.Check
	err := ctx.DB().Model(&models.Check{}).Select("id").
		Where("canary_id = ? AND type = ? AND name = ? AND agent_id = ?", canaryID, checkType, checkName, uuid.Nil).
		First(&check).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return check.ID, fmt.Errorf("check with canary=%s name=%s type=%s not found for local agent", canaryID, checkName, checkType)
		}
		return check.ID, fmt.Errorf("error finding check_id: %w", err)
	}

	_ = c.checkIDCache.Set(ctx, cacheKey, check.ID)
	return check.ID, nil
}
