package db

import (
	"context"
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
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetAllCanariesForSync() ([]pkg.Canary, error) {
	var _canaries []pkg.Canary
	var rawCanaries interface{}
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

	rows, err := Gorm.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&rawCanaries); err != nil {
			return nil, err
		}
	}
	if rawCanaries == nil {
		return nil, nil
	}
	if err := json.Unmarshal(rawCanaries.([]byte), &_canaries); err != nil {
		return nil, err
	}
	return _canaries, nil
}

func GetAllChecks() ([]pkg.Check, error) {
	var checks []pkg.Check
	if err := Gorm.Find(&checks).Error; err != nil {
		return nil, err
	}
	return checks, nil
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

func GetTransformedCheckIDs(canaryID string) ([]string, error) {
	var ids []string
	err := Gorm.Table("checks").
		Select("id").
		Where("canary_id = ? AND transformed = true AND deleted_at IS NULL", canaryID).
		Find(&ids).
		Error
	return ids, err
}

func RemoveTransformedChecks(ids []string, status models.CheckHealthStatus) error {
	if len(ids) == 0 {
		return nil
	}
	updates := map[string]any{
		"deleted_at": gorm.Expr("NOW()"),
	}
	if status != "" {
		if !utils.Contains(models.CheckHealthStatuses, status) {
			return fmt.Errorf("invalid check health status: %s", status)
		}
		updates["status"] = status
	}
	return Gorm.Table("checks").
		Where("id in (?)", ids).
		Where("transformed = true").
		Updates(updates).
		Error
}

func RemoveOldTransformedChecks(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Alertmanager checks are marked as healthy on deletion
	query := `
        UPDATE checks
        SET deleted_at = NOW(),
            status = (CASE WHEN checks.type = 'alertmanager' THEN 'healthy'
                           ELSE checks.status
                      END)
        WHERE id IN (?)
    `
	return Gorm.Exec(query, ids).Error
}

func DeleteCanary(canary v1.Canary) error {
	logger.Infof("deleting canary %s/%s", canary.Namespace, canary.Name)
	model, err := pkg.CanaryFromV1(canary)
	if err != nil {
		return err
	}
	deleteTime := time.Now()
	persistedID := canary.GetPersistedID()
	var checkIDs []string
	for _, checkID := range canary.Status.Checks {
		checkIDs = append(checkIDs, checkID)
	}
	metrics.UnregisterGauge(checkIDs)
	if persistedID == "" {
		logger.Errorf("Canary %s/%s has not been persisted", canary.Namespace, canary.Name)
		return nil
	}
	if err := Gorm.Where("id = ?", persistedID).Find(&model).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	if err := DeleteChecksForCanary(persistedID, deleteTime); err != nil {
		return err
	}
	if err := DeleteCheckComponentRelationshipsForCanary(persistedID, deleteTime); err != nil {
		return err
	}
	return nil
}

func DeleteChecksForCanary(id string, deleteTime time.Time) error {
	return Gorm.Table("checks").Where("canary_id = ? and deleted_at is null", id).UpdateColumn("deleted_at", deleteTime).Error
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
	if err := Gorm.Where("namespace = ? AND name = ?", namespace, name).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}
	return &model, nil
}

func FindCheck(canary pkg.Canary, name string) (*pkg.Check, error) {
	var model pkg.Check
	if err := Gorm.Where("canary_id = ? AND name = ?", canary.ID.String(), name).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func FindDeletedChecksSince(ctx context.Context, since time.Time) ([]string, error) {
	var ids []string
	err := Gorm.Model(&models.Check{}).Where("deleted_at > ?", since).Pluck("id", &ids).Error
	return ids, err
}

func CreateCanary(canary *pkg.Canary) error {
	if canary.Spec == nil || len(canary.Spec) == 0 {
		empty := []byte("{}")
		canary.Spec = types.JSON(empty)
	}

	return Gorm.Create(canary).Error
}

func CreateCheck(canary pkg.Canary, check *pkg.Check) error {
	return Gorm.Create(&check).Error
}

func PersistCanary(canary v1.Canary, source string) (*pkg.Canary, map[string]string, bool, error) {
	changed := false
	model, err := pkg.CanaryFromV1(canary)
	if err != nil {
		return nil, nil, changed, err
	}
	if canary.GetPersistedID() != "" {
		model.ID, _ = uuid.Parse(canary.GetPersistedID())
	}
	model.Source = source
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}, {Name: "name"}, {Name: "namespace"}, {Name: "source"}},
		UpdateAll: true,
	}).Create(&model)
	if tx.RowsAffected > 0 {
		changed = true
	}

	// Duplicate key happens when an already created canary is persisted
	// We will ignore this error but act on other errors
	if err != nil {
		if !errors.Is(tx.Error, gorm.ErrDuplicatedKey) {
			return nil, map[string]string{}, changed, tx.Error
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
		return nil, nil, changed, err
	}

	var checks = make(map[string]string)
	var newCheckIDs []string
	for _, config := range canary.Spec.GetAllChecks() {
		check := pkg.FromExternalCheck(model, config)
		// not creating the new check if already exists in the status
		// status is not patched correctly with the status id
		if checkID := canary.GetCheckID(check.Name); checkID != "" {
			check.ID, _ = uuid.Parse(checkID)
		}
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
	return &model, checks, changed, nil
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
