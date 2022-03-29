package db

import (
	"encoding/json"
	"errors"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllCanaries() ([]v1.Canary, error) {
	var canaries []v1.Canary
	var _canaries []pkg.Canary
	if err := Gorm.Find(&_canaries).Error; err != nil {
		return nil, err
	}
	for _, _canary := range _canaries {
		canary := v1.Canary{
			ObjectMeta: metav1.ObjectMeta{
				Name:      _canary.Name,
				Namespace: _canary.Namespace,
				Labels:    _canary.Labels,
			},
		}
		if err := json.Unmarshal(_canary.Spec, &canary.Spec); err != nil {
			return nil, err
		}
		id := _canary.ID.String()
		canary.Status.PersistedID = &id
		canaries = append(canaries, canary)
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

func PersistCheck(canary pkg.Canary, check external.Check) (string, error) {
	spec, _ := json.Marshal(check)
	name := check.GetName()
	description := check.GetDescription()
	if name == "" {
		name = description
		description = ""
	}

	model := pkg.Check{
		CanaryID:    canary.ID.String(),
		Spec:        spec,
		Type:        check.GetType(),
		Icon:        check.GetIcon(),
		Name:        name,
		Namespace:   canary.Namespace,
		CanaryName:  canary.Name,
		Labels:      canary.Labels,
		Description: description,
	}
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "canary_id"}, {Name: "type"}, {Name: "name"}},
		UpdateAll: true,
	}).Create(&model)
	if tx.Error != nil {
		return "", tx.Error
	}

	return model.ID, nil
}

func DeleteCanary(canary v1.Canary) error {
	return nil
}

func GetCanary(id string) (*pkg.Canary, error) {
	var model *pkg.Canary
	if err := Gorm.Where("id = ?", id).First(model).Error; err != nil {
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

func PersistCanary(canary v1.Canary, source string) (string, error) {
	model := pkg.CanaryFromV1(canary)
	model.Source = source
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, clause.Column{Name: "namespace"}},
		UpdateAll: true,
	}).Create(&model)

	if tx.Error != nil {
		return "", tx.Error
	}

	for _, config := range canary.Spec.GetAllChecks() {
		if _, err := PersistCheck(model, config); err != nil {
			return "", err
		}
	}

	return model.ID.String(), nil
}
