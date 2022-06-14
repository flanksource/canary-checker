package db

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func PersistSystemTemplate(system *v1.SystemTemplate) (string, bool, error) {
	model := pkg.SystemTemplateFromV1(system)
	if system.GetPersistedID() != "" {
		model.ID = uuid.MustParse(system.GetPersistedID())
	}
	var changed bool
	tx := Gorm.Table("templates").Clauses(clause.OnConflict{
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
		_, _, err := PersistSystem(system)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetAllSystemTemplates() ([]v1.SystemTemplate, error) {
	var systemTemplates []v1.SystemTemplate
	var _systemTemplates []pkg.SystemTemplate
	if err := Gorm.Table("templates").Find(&_systemTemplates).Where("deleted_at is NULL").Error; err != nil {
		return nil, err
	}
	for _, _systemTemplate := range _systemTemplates {
		systemTemplates = append(systemTemplates, _systemTemplate.ToV1())
	}
	return systemTemplates, nil
}

// Get all the components from table which has not null selectors
func GetAllComponentWithSelectors() (components pkg.Components, err error) {
	if err := Gorm.Table("components").Where("deleted_at is NULL and selectors != 'null'").Find(&components).Error; err != nil {
		return nil, err
	}
	return
}

func GetComponensWithSelectors(resourceSelectors v1.ResourceSelectors) (components pkg.Components, err error) {
	var uninqueComponents = make(map[string]*pkg.Component)
	for _, resourceSelector := range resourceSelectors {
		if resourceSelector.LabelSelector != "" {
			labelComponents, err := GetComponensWithLabelSelector(resourceSelector.LabelSelector)
			if err != nil {
				continue
			}
			for _, c := range labelComponents {
				uninqueComponents[c.ID.String()] = c
			}
		}
		if resourceSelector.FieldSelector != "" {
			fieldComponents, err := GetComponensWithFieldSelector(resourceSelector.FieldSelector)
			if err != nil {
				continue
			}
			for _, c := range fieldComponents {
				uninqueComponents[c.ID.String()] = c
			}
		}
	}
	for _, comp := range uninqueComponents {
		components = append(components, comp)
	}
	return
}

func GetComponentRelationships(relationshipID uuid.UUID, components pkg.Components) (relationships []*pkg.ComponentRelationship, err error) {
	for _, component := range components {
		relationships = append(relationships, &pkg.ComponentRelationship{
			RelationshipID: relationshipID,
			ComponentID:    component.ID,
			SelectorID:     GetSelectorID(component.Selectors),
		})
	}
	return
}

func GetSelectorID(selectors v1.ResourceSelectors) string {
	data, err := json.Marshal(selectors)
	if err != nil {
		logger.Errorf("Error marshalling selectors %v", err)
		return ""
	}
	hash := md5.Sum(data)
	if err != nil {
		logger.Errorf("Error hashing selector %v", err)
		return ""
	}
	return hex.EncodeToString(hash[:])
}

func PersistSystem(system *pkg.System) (string, []pkg.Component, error) {
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "type"}, {Name: "external_id"}},
		UpdateAll: true,
	}).Create(system)
	if tx.Error != nil {
		return "", nil, tx.Error
	}
	var components []pkg.Component
	for _, component := range system.Components {
		component.SystemId = &system.ID
		var compID string
		var err error
		if compID, err = PersistComponent(component); err != nil {
			logger.Errorf("Error persisting component %v", err)
		}
		components = append(components, *component)
		component.ID = uuid.MustParse(compID)
		for _, child := range component.Components {
			child.SystemId = &system.ID
			child.ParentId = &component.ID
			if _, err := PersistComponent(child); err != nil {
				logger.Errorf("Error persisting child component of %v, :v", component.ID, err)
			}
			components = append(components, *child)
		}
	}
	return system.ID.String(), components, nil
}

func PersisComponentRelationships(relationships []*pkg.ComponentRelationship) error {
	for _, relationship := range relationships {
		if err := PersistComponentRelationship(relationship); err != nil {
			return err
		}
	}
	return nil
}

func PersistComponentRelationship(relationship *pkg.ComponentRelationship) error {
	tx := Gorm.Create(relationship)
	return tx.Error
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

func DeleteSystemTemplate(systemTemplate *v1.SystemTemplate) error {
	logger.Infof("Deleting system template %s/%s", systemTemplate.Namespace, systemTemplate.Name)
	model := pkg.SystemTemplateFromV1(systemTemplate)
	deleteTime := time.Now()
	if systemTemplate.GetPersistedID() == "" {
		logger.Errorf("System template %s/%s has not been persisted", systemTemplate.Namespace, systemTemplate.Name)
		return nil
	}
	tx := Gorm.Table("templates").Find(model, "id = ?", systemTemplate.GetPersistedID()).UpdateColumn("deleted_at", deleteTime)
	if tx.Error != nil {
		return tx.Error
	}
	systemID, err := DeleteSystem(systemTemplate.GetPersistedID(), deleteTime)
	if err != nil {
		return err
	}
	return DeleteComponnents(systemID, deleteTime)
}

func DeleteSystem(systemTemplateID string, deleteTime time.Time) (string, error) {
	logger.Infof("Deleting system associated with template: %s", systemTemplateID)
	systemModel := &pkg.System{}
	tx := Gorm.Find(systemModel).Where("system_template_id = ?", systemTemplateID).UpdateColumn("deleted_at", deleteTime)
	if tx.Error != nil {
		return "", tx.Error
	}
	return systemModel.ID.String(), nil
}

// DeleteComponents deletes all components associated with a system
func DeleteComponnents(systemID string, deleteTime time.Time) error {
	logger.Infof("Deleting all components associated with system: %s", systemID)
	componentsModel := &[]pkg.Component{}
	tx := Gorm.Find(componentsModel).Where("system_id = ?", systemID).UpdateColumn("deleted_at", deleteTime)
	return tx.Error
}

// DeleteComponentsWithID deletes all components with specified ids.
func DeleteComponentsWithID(compId []string, deleteTime time.Time) error {
	logger.Infof("deleting component with id: %s", compId)
	componentsModel := &[]pkg.Component{}
	tx := Gorm.Find(componentsModel).Where("id in (?)", compId).UpdateColumn("deleted_at", deleteTime)
	return tx.Error
}

func GetComponentsWithSystemID(systemID string) ([]pkg.Component, error) {
	logger.Infof("Finding components with system id: %s", systemID)
	componentsModel := &[]pkg.Component{}
	tx := Gorm.Find(componentsModel).Where("system_id = ?", systemID)
	return *componentsModel, tx.Error
}
