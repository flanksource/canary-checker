package db

import (
	"encoding/json"
	"errors"
	"fmt"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	dutyTypes "github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetAllCanariesForSync(ctx context.Context, namespace string) ([]pkg.Canary, error) {
	query := `
        SELECT json_agg(
            jsonb_set_lax(to_jsonb(canaries),'{checks}', (
                SELECT json_object_agg(checks.name, checks.id)
                FROM checks
                WHERE
                    checks.canary_id = canaries.id
                    AND checks.deleted_at IS NULL
                GROUP BY checks.canary_id
                ) :: jsonb
            )
        ) :: jsonb AS canaries
        FROM canaries
        WHERE
            deleted_at IS NULL AND
            agent_id = '00000000-0000-0000-0000-000000000000'
    `

	args := make(pgx.NamedArgs)

	if namespace != "" {
		query += " AND namespace = @namespace"
		args["namespace"] = namespace
	}

	rows, err := ctx.Pool().Query(ctx, query, args)
	if err != nil {
		return nil, err
	}

	var _canaries []pkg.Canary
	for rows.Next() {
		if rows.RawValues()[0] == nil {
			continue
		}

		if err := json.Unmarshal(rows.RawValues()[0], &_canaries); err != nil {
			return nil, fmt.Errorf("failed to unmarshal canaries:%w for %s", err, rows.RawValues()[0])
		}
	}

	return _canaries, nil
}

func PersistCheck(check pkg.Check, canaryID uuid.UUID) (uuid.UUID, error) {
	check.CanaryID = canaryID
	name := check.GetName()
	description := check.GetDescription()
	if name == "" {
		name = description
		description = ""
	}
	check.Name = name
	check.Description = description

	// TODO: Find root cause why pod check has these labels in check model
	delete(check.Labels, "canary-checker.flanksource.com/podCheck")
	delete(check.Labels, "canary-checker.flanksource.com/check")

	delete(check.Labels, "controller-revision-hash")

	assignments := map[string]interface{}{
		"spec":        check.Spec,
		"type":        check.Type,
		"description": check.Description,
		"owner":       check.Owner,
		"severity":    check.Severity,
		"icon":        check.Icon,
		"labels":      check.Labels,
		"deleted_at":  nil,
	}

	if err := Gorm.Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "canary_id"}, {Name: "type"}, {Name: "name"}, {Name: "agent_id"}},
			DoUpdates: clause.Assignments(assignments),
		},
	).Create(&check).Error; err != nil {
		return uuid.Nil, err
	}

	// There are cases where we may receive a transformed check with a nil uuid
	// We then explicitly query for that ID using the unique fields we have
	if check.ID == uuid.Nil {
		var err error
		var idStr string
		if err := Gorm.Table("checks").Select("id").Where("canary_id = ? AND type = ? AND name = ? AND agent_id = ?", check.CanaryID, check.Type, check.Name, uuid.Nil).Find(&idStr).Error; err != nil {
			return uuid.Nil, err
		}
		check.ID, err = uuid.Parse(idStr)
		if err != nil {
			return uuid.Nil, err
		}
	}

	if check.ID == uuid.Nil {
		return check.ID, fmt.Errorf("received nil check id for canary:%s", canaryID)
	}
	return check.ID, nil
}

func GetTransformedCheckIDs(ctx context.Context, canaryID string) ([]string, error) {
	var ids []string
	err := ctx.DB().Table("checks").
		Select("id").
		Where("canary_id = ? AND transformed = true AND deleted_at IS NULL", canaryID).
		Find(&ids).
		Error
	return ids, err
}

func AddCheckStatuses(ctx context.Context, ids []string, status models.CheckHealthStatus) error {
	if len(ids) == 0 {
		return nil
	}
	if status == "" || !utils.Contains(models.CheckHealthStatuses, status) {
		return fmt.Errorf("invalid check health status: %s", status)
	}
	checkStatus := false
	if status == models.CheckStatusHealthy {
		checkStatus = true
	}

	var objs []*models.CheckStatus
	for _, id := range ids {
		if checkID, err := uuid.Parse(id); err != nil {
			objs = append(objs, &models.CheckStatus{
				CheckID:   checkID,
				Time:      time.Now().UTC().Format(time.RFC3339),
				CreatedAt: time.Now(),
				Status:    checkStatus,
			})
		}
	}
	return ctx.DB().Table("check_statuses").
		Create(objs).
		Error
}

func RemoveTransformedChecks(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	updates := map[string]any{
		"deleted_at": gorm.Expr("NOW()"),
	}

	return ctx.DB().Table("checks").
		Where("id in (?)", ids).
		Where("transformed = true").
		Updates(updates).
		Error
}

func DeleteCanary(id string, deleteTime time.Time) error {
	logger.Infof("Deleting canary[%s]", id)

	if err := Gorm.Table("canaries").Where("id = ?", id).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	checkIDs, err := DeleteChecksForCanary(id, deleteTime)
	if err != nil {
		return err
	}
	metrics.UnregisterGauge(checkIDs)

	if err := DeleteCheckComponentRelationshipsForCanary(id, deleteTime); err != nil {
		return err
	}
	return nil
}

func DeleteChecksForCanary(id string, deleteTime time.Time) ([]string, error) {
	var checkIDs []string
	var checks []pkg.Check
	err := Gorm.Model(&checks).
		Table("checks").
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		Where("canary_id = ? and deleted_at IS NULL", id).
		UpdateColumn("deleted_at", deleteTime).
		Error

	for _, c := range checks {
		checkIDs = append(checkIDs, c.ID.String())
	}
	return checkIDs, err
}

func DeleteCheckComponentRelationshipsForCanary(id string, deleteTime time.Time) error {
	return Gorm.Table("check_component_relationships").Where("canary_id = ?", id).UpdateColumn("deleted_at", deleteTime).Error
}

func DeleteNonTransformedChecks(id []string) error {
	return Gorm.Table("checks").Where("id IN (?) and transformed = false", id).UpdateColumn("deleted_at", time.Now()).Error
}

func GetCanary(id string) (pkg.Canary, error) {
	var model pkg.Canary
	err := Gorm.Where("id = ?", id).First(&model).Error
	return model, err
}

func FindCanaryByID(id string) (*pkg.Canary, error) {
	var model *pkg.Canary
	if err := Gorm.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return model, nil
}

func GetCheck(id string) (*pkg.Check, error) {
	var model *pkg.Check
	if err := Gorm.Where("id = ?", id).First(model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model, nil
}

func FindCanary(namespace, name string) (*pkg.Canary, error) {
	var model pkg.Canary
	if err := Gorm.
		Where("namespace = ? AND name = ?", namespace, name).
		Where("agent_id = '00000000-0000-0000-0000-000000000000'").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}
	return &model, nil
}

func FindCheck(canary pkg.Canary, name string) (*pkg.Check, error) {
	var model pkg.Check
	if err := Gorm.Where("canary_id = ? AND name = ?", canary.ID.String(), name).
		Where("agent_id = '00000000-0000-0000-0000-000000000000'").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func FindDeletedChecksSince(ctx context.Context, since time.Time) ([]string, error) {
	var ids []string
	err := ctx.DB().Model(&models.Check{}).Where("deleted_at > ?", since).Pluck("id", &ids).Error
	return ids, err
}

func CreateCanary(canary *pkg.Canary) error {
	if canary.Spec == nil || len(canary.Spec) == 0 {
		empty := []byte("{}")
		canary.Spec = dutyTypes.JSON(types.JSON(empty))
	}

	return Gorm.Create(canary).Error
}

func CreateCheck(canary pkg.Canary, check *pkg.Check) error {
	return Gorm.Create(&check).Error
}

func PersistCanaryModel(model pkg.Canary) (*pkg.Canary, error) {
	err := Gorm.Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_id"}, {Name: "name"}, {Name: "namespace"}, {Name: "source"}},
			DoUpdates: clause.AssignmentColumns([]string{"labels", "spec"}),
		},
		clause.Returning{},
	).Create(&model).Error

	// Duplicate key happens when an already created canary is persisted
	// We will ignore this error but act on other errors
	if err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, err
		}
	}

	var oldCheckIDs []string
	err = Gorm.
		Table("checks").
		Select("id").
		Where("canary_id = ? AND deleted_at IS NULL AND transformed = false", model.ID).
		Scan(&oldCheckIDs).
		Error
	if err != nil {
		logger.Errorf("Error fetching existing checks for canary:%s", model.ID)
		return nil, err
	}

	var spec v1.CanarySpec
	if err = json.Unmarshal(model.Spec, &spec); err != nil {
		return nil, err
	}

	var checks = make(map[string]string)
	var newCheckIDs []string
	for _, config := range spec.GetAllChecks() {
		check := pkg.FromExternalCheck(model, config)
		check.Spec, _ = json.Marshal(config)
		id, err := PersistCheck(check, model.ID)
		if err != nil {
			logger.Errorf("error persisting check", err)
		}
		newCheckIDs = append(newCheckIDs, id.String())
		checks[config.GetName()] = id.String()
	}

	// Delete non-transformed checks which are no longer in the canary
	// fetching the checkIds present in the db but not present on the canary
	checkIDsToRemove := utils.SetDifference(oldCheckIDs, newCheckIDs)
	if len(checkIDsToRemove) > 0 {
		logger.Infof("removing checks from canary:%s with ids %v", model.ID, checkIDsToRemove)
		if err := DeleteNonTransformedChecks(checkIDsToRemove); err != nil {
			logger.Errorf("failed to delete non transformed checks: %v", err)
		}
		metrics.UnregisterGauge(checkIDsToRemove)
	}

	model.Checks = checks
	return &model, nil
}

func PersistCanary(canary v1.Canary, source string) (*pkg.Canary, error) {
	model, err := pkg.CanaryFromV1(canary)
	if err != nil {
		return nil, err
	}
	if canary.GetPersistedID() != "" {
		model.ID, _ = uuid.Parse(canary.GetPersistedID())
	}
	model.Source = source

	return PersistCanaryModel(model)
}

func RefreshCheckStatusSummary() {
	if err := duty.RefreshCheckStatusSummary(Pool); err != nil {
		logger.Errorf("error refreshing check_status_summary materialized view: %v", err)
	}
}

const (
	DefaultCheckRetentionDays  = 7
	DefaultCanaryRetentionDays = 7
)

var (
	CheckRetentionDays  int
	CanaryRetentionDays int
)

func CleanupChecks() {
	jobHistory := models.NewJobHistory("CleanupChecks", "checks", "").Start()
	_ = PersistJobHistory(jobHistory)
	defer func() {
		_ = PersistJobHistory(jobHistory.End())
	}()

	if CheckRetentionDays <= 0 {
		CheckRetentionDays = DefaultCheckRetentionDays
	}
	err := Gorm.Exec(`
        DELETE FROM checks
        WHERE
            id NOT IN (SELECT check_id FROM evidences WHERE check_id IS NOT NULL) AND
            (NOW() - deleted_at) > INTERVAL '1 day' * ?
        `, CheckRetentionDays).Error
	if err != nil {
		logger.Errorf("Error cleaning up checks: %v", err)
		jobHistory.AddError(err.Error())
	} else {
		jobHistory.IncrSuccess()
	}
}

func CleanupCanaries() {
	jobHistory := models.NewJobHistory("CleanupCanaries", "canaries", "").Start()
	_ = PersistJobHistory(jobHistory)
	defer func() {
		_ = PersistJobHistory(jobHistory.End())
	}()

	if CanaryRetentionDays <= 0 {
		CanaryRetentionDays = DefaultCanaryRetentionDays
	}
	err := Gorm.Exec(`
        DELETE FROM canaries
        WHERE
            id NOT IN (SELECT canary_id FROM checks) AND
            (NOW() - deleted_at) > INTERVAL '1 day' * ?
        `, CanaryRetentionDays).Error

	if err != nil {
		logger.Errorf("Error cleaning up canaries: %v", err)
		jobHistory.AddError(err.Error())
	} else {
		jobHistory.IncrSuccess()
	}
}
