package db

import (
	"encoding/json"
	"errors"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/commons/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"helm.sh/helm/v3/pkg/time"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllCanaries() ([]v1.Canary, error) {
	var canaries []v1.Canary
	var _canaries []pkg.Canary
	if err := Gorm.Find(&_canaries).Where("deleted_at = NULL").Error; err != nil {
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
		var deletionTimestamp metav1.Time
		if _canary.DeletedAt.Valid {
			deletionTimestamp = metav1.NewTime(_canary.DeletedAt.Time)
			canary.ObjectMeta.DeletionTimestamp = &deletionTimestamp
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
	model := pkg.CanaryFromV1(canary)
	deleteTime := time.Now().Time
	tx := Gorm.Find(&model).Where("id = ?", *canary.Status.PersistedID).UpdateColumn("deleted_at", deleteTime)
	if tx.Error != nil {
		return tx.Error
	}
	var checkmodel = &pkg.Check{}
	tx = Gorm.Find(&checkmodel).Where("canary_id = ?", model.ID.String()).UpdateColumn("deleted_at", deleteTime)
	return tx.Error
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

func PersistCanary(canary v1.Canary, source string) (string, bool, error) {
	changed := false
	model := pkg.CanaryFromV1(canary)
	model.Source = source
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, clause.Column{Name: "namespace"}},
		UpdateAll: true,
	}).Create(&model)
	if tx.RowsAffected > 0 {
		changed = true
	}
	if tx.Error != nil {
		return "", changed, tx.Error
	}

	for _, config := range canary.Spec.GetAllChecks() {
		if _, err := PersistCheck(pkg.FromExternalCheck(model, config)); err != nil {
			return "", changed, err
		}
	}

	return model.ID.String(), changed, nil
}
