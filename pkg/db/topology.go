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

func PersistComponents(results []*pkg.Component) error {
	for _, component := range results {
		_, err := PersistComponent(component)
		if err != nil {
			logger.Debugf("Error persisting component %v", err)
			continue
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

func GetComponentRelationships(relationshipID uuid.UUID, path string, components pkg.Components) (relationships []*pkg.ComponentRelationship, err error) {
	for _, component := range components {
		relationships = append(relationships, &pkg.ComponentRelationship{
			RelationshipID:   relationshipID,
			ComponentID:      component.ID,
			SelectorID:       GetSelectorID(component.Selectors),
			RelationshipPath: path + "." + relationshipID.String(),
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

func PersisComponentRelationships(relationships []*pkg.ComponentRelationship) error {
	for _, relationship := range relationships {
		if err := PersistComponentRelationship(relationship); err != nil {
			return err
		}
	}
	return nil
}

func PersistComponentRelationship(relationship *pkg.ComponentRelationship) error {
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "component_id"}, {Name: "relationship_id"}, {Name: "selector_id"}},
		UpdateAll: true,
	}).Create(relationship)
	return tx.Error
}

func PersistComponent(component *pkg.Component) ([]uuid.UUID, error) {
	existingComponenet := &pkg.Component{}
	var persistedComponents []uuid.UUID
	var tx *gorm.DB
	if component.ParentId == nil {
		tx = Gorm.Find(existingComponenet, "system_template_id = ? AND name = ? AND type = ? and parent_id is NULL", component.SystemTemplateID, component.Name, component.Type)
	} else {
		tx = Gorm.Find(existingComponenet, "system_template_id = ? AND name = ? AND type = ? and parent_id = ?", component.SystemTemplateID, component.Name, component.Type, component.ParentId)
	}
	if existingComponenet.ID != uuid.Nil {
		component.ID = existingComponenet.ID
		tx.UpdateColumns(component)
	} else {
		tx = Gorm.Create(component)
	}
	persistedComponents = append(persistedComponents, component.ID)
	for _, child := range component.Components {
		child.SystemTemplateID = component.SystemTemplateID
		if component.Path != "" {
			child.Path = component.Path + "." + component.ID.String()
		} else {
			child.Path = component.ID.String()
		}
		child.ParentId = &component.ID
		if childIDs, err := PersistComponent(child); err != nil {
			logger.Errorf("Error persisting child component of %v, :v", component.ID, err)
		} else {
			persistedComponents = append(persistedComponents, childIDs...)
		}
	}
	return persistedComponents, tx.Error
}

func UpdateStatusAndSummarForComponent(id uuid.UUID, status string, summary v1.Summary) error {
	component := &pkg.Component{}
	tx := Gorm.Where("id = ?", id).Find(component)
	if tx.Error != nil {
		return tx.Error
	}
	component.Status = status
	component.Summary = summary
	tx = Gorm.Save(component)
	return tx.Error
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
	return DeleteComponnents(systemTemplate.GetPersistedID(), deleteTime)
}

// DeleteComponents deletes all components associated with a systemTemplate
func DeleteComponnents(systemTemplateID string, deleteTime time.Time) error {
	logger.Infof("Deleting all components associated with system: %s", systemTemplateID)
	componentsModel := &[]pkg.Component{}
	tx := Gorm.Find(componentsModel).Where("system_template_id = ?", systemTemplateID).UpdateColumn("deleted_at", deleteTime)
	DeleteComponentRelationshipForComponents(componentsModel, deleteTime)
	return tx.Error
}

func DeleteComponentRelationshipForComponents(components *[]pkg.Component, deleteTime time.Time) {
	for _, component := range *components {
		if err := DeleteComponentRelationship(component.ID.String(), deleteTime); err != nil {
			logger.Debugf("Error deleting component relationship for component %v", component.ID)
		}
	}
}

func DeleteComponentRelationship(componentID string, deleteTime time.Time) error {
	logger.Infof("Deleting component relationship for components %s", componentID)
	tx := Gorm.Delete(&pkg.ComponentRelationship{}, "component_id = ? or relationship_id = ?", componentID, componentID).UpdateColumn("deleted_at", deleteTime)
	return tx.Error
}

// DeleteComponentsWithID deletes all components with specified ids.
func DeleteComponentsWithIDs(compIDs []string, deleteTime time.Time) error {
	logger.Infof("deleting component with ids: %v", compIDs)
	componentsModel := &[]pkg.Component{}
	tx := Gorm.Find(componentsModel).Where("id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime)
	return tx.Error
}

func GetActiveComponentsIDsWithSystemTemplateID(systemID string) (compIDs []uuid.UUID, err error) {
	logger.Infof("Finding components with system id: %s", systemID)
	if err := Gorm.Table("components").Where("deleted_at is NULL and system_template_id = ?", systemID).Select("id").Find(&compIDs).Error; err != nil {
		return nil, err
	}
	return
}
