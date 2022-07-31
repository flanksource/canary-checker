package db

import (
	"encoding/json"
	"errors"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetAllCanaries() ([]v1.Canary, error) {
	var canaries []v1.Canary
	var _canaries []pkg.Canary
	if err := Gorm.Where("deleted_at is NULL").Find(&_canaries).Error; err != nil {
		return nil, err
	}
	for _, _canary := range _canaries {
		canaries = append(canaries, *_canary.ToV1())
	}
	return canaries, nil
}

func GetAllChecks() ([]pkg.Check, error) {
	var checks []pkg.Check
	if err := Gorm.Find(&checks).Error; err != nil {
		return nil, err
	}
	return checks, nil
}

func PersistCheck(check pkg.Check) (string, error) {
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
		Columns:   []clause.Column{{Name: "canary_id"}, {Name: "type"}, {Name: "name"}},
		UpdateAll: true,
	}).Create(&check)
	if tx.Error != nil {
		return "", tx.Error
	}

	return check.ID, nil
}

func DeleteCanary(canary v1.Canary) error {
	logger.Infof("deleting canary %s/%s", canary.Namespace, canary.Name)
	model, err := pkg.CanaryFromV1(canary)
	if err != nil {
		return err
	}
	deleteTime := time.Now()
	persistedID := canary.GetPersistedID()
	if persistedID == "" {
		logger.Errorf("System template %s/%s has not been persisted", canary.Namespace, canary.Name)
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
	return Gorm.Table("checks").Where("canary_id = ?", id).UpdateColumn("deleted_at", deleteTime).Error
}

func DeleteCheckComponentRelationshipsForCanary(id string, deleteTime time.Time) error {
	return Gorm.Table("check_component_relationships").Where("canary_id = ?", id).UpdateColumn("deleted_at", deleteTime).Error
}

func GetCanary(id string) (*pkg.Canary, error) {
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

func PersistCanary(canary v1.Canary, source string) (string, bool, error) {
	changed := false
	model, err := pkg.CanaryFromV1(canary)
	if err != nil {
		return "", changed, err
	}
	if canary.GetPersistedID() != "" {
		model.ID = uuid.MustParse(canary.GetPersistedID())
	}
	model.Source = source
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "namespace"}, {Name: "source"}},
		UpdateAll: true,
	}).Create(&model)
	if tx.RowsAffected > 0 {
		changed = true
	}
	if tx.Error != nil {
		return "", changed, tx.Error
	}

	return model.ID.String(), changed, nil
}
