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

func GetAllCanaries() ([]pkg.Canary, error) {
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
        FROM canaries WHERE deleted_at IS NULL`

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

	// Deletions for transformed checks are handled separately
	if check.Transformed {
		delete(assignments, "deleted_at")
	}
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "canary_id"}, {Name: "type"}, {Name: "name"}, {Name: "agent_id"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(&check)
	if tx.Error != nil {
		return uuid.Nil, tx.Error
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

func UpdateChecksStatus(ids []string, status models.CheckHealthStatus) error {
	if len(ids) == 0 {
		return nil
	}
	if !utils.Contains(models.CheckHealthStatuses, status) {
		return fmt.Errorf("invalid check health status: %s", status)
	}
	return Gorm.Table("checks").
		Where("id in (?)", ids).
		Updates(map[string]any{
			"status":     status,
			"deleted_at": gorm.Expr("NOW()"),
		}).
		Error
}

// unregisterChecksGauge unregisters all the checks of the associated with canary
func unregisterChecksGauge(canaryID string) error {
	var checkIDs []string
	if err := Gorm.Model(&models.Check{}).Select("id").Where("canary_id = ?", canaryID).Pluck("id", &checkIDs).Error; err != nil {
		return err
	}

	metrics.UnregisterGauge(checkIDs)
	return nil
}

func deleteCanary(canaryID string, canary pkg.Canary) error {
	deleteTime := time.Now()

	if err := Gorm.Where("id = ?", canaryID).Find(&canary).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return fmt.Errorf("failed to delete canary %s: %v", canaryID, err)
	}

	if err := DeleteChecksForCanary(canaryID, deleteTime); err != nil {
		return fmt.Errorf("failed to delete checks for canary %s: %v", canaryID, err)
	}

	if err := DeleteCheckComponentRelationshipsForCanary(canaryID, deleteTime); err != nil {
		return fmt.Errorf("failed to delete check component relationships for canary %s: %v", canaryID, err)
	}

	return nil
}

// DeleteCanaryByID deletes a canary and the associated checks and relationships.
func DeleteCanaryByID(canaryID string) error {
	if err := unregisterChecksGauge(canaryID); err != nil {
		logger.Errorf("failed to unregister checks gauges: %v", err)
	}

	return deleteCanary(canaryID, pkg.Canary{})
}

// DeleteCanary deletes the canary by the given filter.
func DeleteCanary(canary v1.Canary) error {
	logger.Infof("deleting canary %s/%s", canary.Namespace, canary.Name)

	model, err := pkg.CanaryFromV1(canary)
	if err != nil {
		return err
	}

	var checkIDs []string
	for _, checkID := range canary.Status.Checks {
		checkIDs = append(checkIDs, checkID)
	}
	metrics.UnregisterGauge(checkIDs)

	id := canary.GetPersistedID()
	if id == "" {
		logger.Errorf("Canary %s/%s has not been persisted", canary.Namespace, canary.Name)
		return nil
	}

	return deleteCanary(id, model)
}

func DeleteChecksForCanary(id string, deleteTime time.Time) error {
	return Gorm.Table("checks").Where("canary_id = ? and deleted_at is null", id).UpdateColumn("deleted_at", deleteTime).Error
}

func DeleteCheckComponentRelationshipsForCanary(id string, deleteTime time.Time) error {
	return Gorm.Table("check_component_relationships").Where("canary_id = ?", id).UpdateColumn("deleted_at", deleteTime).Error
}

func DeleteChecks(id []string) error {
	return Gorm.Table("checks").Where("id IN (?)", id).UpdateColumn("deleted_at", time.Now()).Error
}

func GetCanary(id string) (*pkg.Canary, error) {
	var model *pkg.Canary
	if err := Gorm.Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}

	return model, nil
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

	var checks = make(map[string]string)

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
		checks[config.GetName()] = id.String()
	}

	return &model, checks, changed, nil
}

func RefreshCheckStatusSummary() {
	if err := duty.RefreshCheckStatusSummary(Pool); err != nil {
		logger.Errorf("error refreshing check_status_summary materialized view: %v", err)
	}
}
