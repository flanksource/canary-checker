package db

import (
	"encoding/json"
	"errors"
	"fmt"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/models"
	dutyTypes "github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetAllCanaries() ([]models.Canary, error) {
	var _canaries []models.Canary
	var rawCanaries interface{}
	query := fmt.Sprintf("SELECT json_agg(jsonb_set_lax(to_jsonb(canaries),'{checks}', %s)) :: jsonb as canaries from canaries where deleted_at is null", getChecksForCanaries())

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
	if check.Spec == nil {
		spec, _ := json.Marshal(check)
		check.Spec = spec
	}
	name := check.GetName()
	description := check.GetDescription()
	if name == "" {
		name = description
		description = ""
	}
	check.Name = name
	check.Description = description
	tx := Gorm.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "canary_id"}, {Name: "type"}, {Name: "name"}},
		DoUpdates: clause.Assignments(
			map[string]interface{}{
				"spec":        check.Spec,
				"type":        check.Type,
				"description": check.Description,
				"owner":       check.Owner,
				"severity":    check.Severity,
				"icon":        check.Icon,
				"labels":      check.Labels,
				"deleted_at":  nil,
			}),
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
		Where("canary_id = ? AND transformed = true", canaryID).
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
		Update("status", status).
		Error
}

func DeleteCanary(canary v1.Canary) error {
	logger.Infof("deleting canary %s/%s", canary.Namespace, canary.Name)
	model, err := canary.ToModel()
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

func DeleteChecks(id []string) error {
	return Gorm.Table("checks").Where("id IN (?)", id).UpdateColumn("deleted_at", time.Now()).Error
}

func GetCanary(id string) (*models.Canary, error) {
	var model *models.Canary
	if err := Gorm.Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}

	return model, nil
}

func FindCanaryByID(id string) (*models.Canary, error) {
	var model *models.Canary
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

func FindCanary(namespace, name string) (*models.Canary, error) {
	var model models.Canary
	if err := Gorm.Where("namespace = ? AND name = ?", namespace, name).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}
	return &model, nil
}

func FindCheck(canary models.Canary, name string) (*pkg.Check, error) {
	var model pkg.Check
	if err := Gorm.Where("canary_id = ? AND name = ?", canary.ID.String(), name).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func CreateCanary(canary *models.Canary) error {
	if canary.Spec == nil || len(canary.Spec) == 0 {
		empty := []byte("{}")
		canary.Spec = dutyTypes.JSON(empty)
	}

	return Gorm.Create(canary).Error
}

func CreateCheck(canary models.Canary, check *pkg.Check) error {
	return Gorm.Create(&check).Error
}

func PersistCanary(canary v1.Canary, source string) (*models.Canary, map[string]string, bool, error) {
	changed := false
	model, err := canary.ToModel()
	if err != nil {
		return nil, nil, changed, err
	}
	if canary.GetPersistedID() != "" {
		model.ID, _ = uuid.Parse(canary.GetPersistedID())
	}
	model.Source = source
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "namespace"}, {Name: "source"}},
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
		id, err := PersistCheck(check, model.ID)
		if err != nil {
			logger.Errorf("error persisting check", err)
		}
		checks[config.GetName()] = id.String()
	}

	return &model, checks, changed, nil
}

func getChecksForCanaries() string {
	return `
	(SELECT json_object_agg(checks.name, checks.id) from checks WHERE checks.canary_id = canaries.id AND checks.deleted_at is null GROUP BY checks.canary_id) :: jsonb
			 `
}

func RefreshCheckStatusSummary() {
	if err := duty.RefreshCheckStatusSummary(Pool); err != nil {
		logger.Errorf("error refreshing check_status_summary materialized view: %v", err)
	}
}
