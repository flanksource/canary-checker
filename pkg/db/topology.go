package db

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func AddSystemTemplate(system *v1.SystemTemplate) (string, bool, error) {
	model := pkg.SystemTemplateFromV1(system)
	var changed bool
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "namespace"}},
		UpdateAll: true,
	}).Create(model)
	if tx.Error != nil {
		return "", changed, tx.Error
	}
	if tx.RowsAffected > 0 {
		changed = true
	}
	return model.ID.String(), changed, nil
}

func PersistSystems(results []*pkg.System) error {
	for _, system := range results {
		err := PersistSystem(system)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetAllSystemTemplates() ([]v1.SystemTemplate, error) {
	var systemTemplates []v1.SystemTemplate
	var _systemTemplates []pkg.SystemTemplate
	if err := Gorm.Find(&_systemTemplates).Error; err != nil {
		return nil, err
	}
	for _, _systemTemplate := range _systemTemplates {
		systemTemplates = append(systemTemplates, _systemTemplate.ToV1())
	}
	return systemTemplates, nil
}

func PersistSystem(system *pkg.System) error {
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "type"}, {Name: "external_id"}},
		UpdateAll: true,
	}).Create(system)
	if tx.Error != nil {
		return tx.Error
	}
	for _, component := range system.Components {
		component.SystemId = &system.ID
		var compID string
		var err error
		if compID, err = PersistComponent(component); err != nil {
			logger.Errorf("Error persisting component %v", err)
		}
		component.ID = uuid.MustParse(compID)
		for _, child := range component.Components {
			child.SystemId = &system.ID
			child.ParentId = &component.ID
			if _, err := PersistComponent(child); err != nil {
				logger.Errorf("Error persisting child component of %v, :v", component.ID, err)
			}
		}
	}
	return nil
}

func PersistComponent(component *pkg.Component) (string, error) {
	existingComponenet := &pkg.Component{}
	var tx *gorm.DB
	if component.ParentId == nil {
		tx = Gorm.Find(existingComponenet, "system_id = ? AND name = ? AND type = ? and parent_id is NULL", component.SystemId, component.Name, component.Type)
	} else {
		tx = Gorm.Find(existingComponenet, "system_id = ? AND name = ? AND type = ? and parent_id = ?", component.SystemId, component.Name, component.Type, component.ParentId)
	}
	if existingComponenet.ID != uuid.Nil {
		component.ID = existingComponenet.ID
		tx.UpdateColumns(component)
	} else {
		tx = Gorm.Create(component)
	}
	return component.ID.String(), tx.Error
}
