package db

import (
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func PersistSystemTemplate(system *v1.SystemTemplate) (string, bool, error) {
	model := pkg.SystemTemplateFromV1(system)
	if system.GetPersistedID() != "" {
		model.ID, _ = uuid.Parse(system.GetPersistedID())
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

func GetComponentsWithSelectors(resourceSelectors v1.ResourceSelectors) (components pkg.Components, err error) {
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

func GetAllComponentsWithConfigs() (components pkg.Components, err error) {
	if err := Gorm.Table("components").Where("deleted_at is NULL and configs != 'null'").Find(&components).Error; err != nil {
		return nil, err
	}
	return
}

func GetAllComponentWithCanaries() (components pkg.Components, err error) {
	if err := Gorm.Table("components").Where("deleted_at is NULL and component_checks != 'null'").Find(&components).Error; err != nil {
		return nil, err
	}
	return
}

func NewComponentRelationships(relationshipID uuid.UUID, path string, components pkg.Components) (relationships []*pkg.ComponentRelationship, err error) {
	for _, component := range components {
		selectorID, err := utils.GenerateJSONMD5Hash(component.Selectors)
		if err != nil {
			logger.Errorf("Error generationg selector_id hash: %v", err)
		}

		relationships = append(relationships, &pkg.ComponentRelationship{
			RelationshipID:   relationshipID,
			ComponentID:      component.ID,
			SelectorID:       selectorID,
			RelationshipPath: path + "." + relationshipID.String(),
		})
	}
	return
}

func GetChildRelationshipsForParentComponent(componentID uuid.UUID) ([]pkg.ComponentRelationship, error) {
	var relationships []pkg.ComponentRelationship
	if err := Gorm.Where("relationship_id = ? AND deleted_at IS NOT NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
}

func PersistComponentRelationships(parentComponentID uuid.UUID, relationships []*pkg.ComponentRelationship) error {
	var selectorIDs, childComponentIDs []string

	existingRelationShips, err := GetChildRelationshipsForParentComponent(parentComponentID)
	if err != nil {
		return err
	}
	for _, r := range existingRelationShips {
		selectorIDs = append(selectorIDs, r.SelectorID)
		childComponentIDs = append(childComponentIDs, r.ComponentID.String())
	}

	var newChildComponentIDs []string
	for _, r := range relationships {
		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, r.SelectorID) {
			continue
		}

		// If childComponentID does not exist, create a new relationship
		if !collections.Contains(childComponentIDs, r.ComponentID.String()) {
			if err := PersistComponentRelationship(r); err != nil {
				return errors.Wrap(err, "error persisting component relationships")
			}
			newChildComponentIDs = append(newChildComponentIDs, r.ComponentID.String())
		}

		// If childComponentID exists mark old row as deleted and update selector_id
		if err := Gorm.Model(&pkg.ComponentRelationship{}).Where("relationship_id = ? AND component_id = ?", parentComponentID, r.ComponentID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating component relationships")
		}

		if err := PersistComponentRelationship(r); err != nil {
			return err
		}
		newChildComponentIDs = append(newChildComponentIDs, r.ComponentID.String())
	}

	// Take set difference of these child component Ids and delete them
	childComponentIDsToDelete := utils.SetDifference(childComponentIDs, newChildComponentIDs)
	if err := Gorm.Model(&pkg.ComponentRelationship{}).Where("relationship_id = ? AND component_id IN ?", parentComponentID, childComponentIDsToDelete).
		Update("deleted_at", time.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale component relationships")
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

func GetCheckRelationshipsForComponent(componentID uuid.UUID) ([]pkg.CheckComponentRelationship, error) {
	var relationships []pkg.CheckComponentRelationship
	if err := Gorm.Where("component_id = ? AND deleted_at IS NOT NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
}

func PersistCheckComponentRelationshipsForComponent(componentID uuid.UUID, relationships []*pkg.CheckComponentRelationship) error {
	var selectorIDs, checkIDs []string
	existingRelationShips, err := GetCheckRelationshipsForComponent(componentID)
	if err != nil {
		return err
	}
	for _, r := range existingRelationShips {
		selectorIDs = append(selectorIDs, r.SelectorID)
		checkIDs = append(checkIDs, r.CheckID.String())
	}

	var newCheckIDs []string
	for _, r := range relationships {
		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, r.SelectorID) {
			continue
		}

		// If checkID does not exist, create a new relationship
		if !collections.Contains(checkIDs, r.CheckID.String()) {
			if err := PersistCheckComponentRelationship(r); err != nil {
				return errors.Wrap(err, "error persisting check component relationships")
			}
			newCheckIDs = append(newCheckIDs, r.CheckID.String())
		}

		// If check_id exists mark old row as deleted and update selector_id
		if err := Gorm.Model(&pkg.CheckComponentRelationship{}).Where("component_id = ? AND check_id = ?", componentID, r.CheckID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating check relationships")
		}

		if err := PersistCheckComponentRelationship(r); err != nil {
			return errors.Wrap(err, "error persisting check component relationships")
		}
		newCheckIDs = append(newCheckIDs, r.CheckID.String())
	}

	// Take set difference of these child component Ids and delete them
	checkIDsToDelete := utils.SetDifference(checkIDs, newCheckIDs)
	if err := Gorm.Model(&pkg.CheckComponentRelationship{}).Where("component_id = ? AND check_id IN ?", componentID, checkIDsToDelete).
		Update("deleted_at", time.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale check component relationships")
	}

	return nil
}

func PersistCheckComponentRelationship(relationship *pkg.CheckComponentRelationship) error {
	tx := Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "canary_id"}, {Name: "check_id"}, {Name: "component_id"}, {Name: "selector_id"}},
		UpdateAll: true,
	}).Create(relationship)
	return tx.Error
}

func PersistComponent(component *pkg.Component) ([]uuid.UUID, error) {
	existing := &pkg.Component{}
	var persisted []uuid.UUID
	var tx *gorm.DB
	if component.SystemTemplateID == nil {
		if component.ParentId == nil {
			tx = Gorm.Find(existing, "name = ? AND type = ? and parent_id is NULL", component.Name, component.Type)
		} else {
			tx = Gorm.Find(existing, "name = ? AND type = ? and parent_id = ?", component.Name, component.Type, component.ParentId)
		}
	} else {
		if component.ParentId == nil {
			tx = Gorm.Find(existing, "system_template_id = ? AND name = ? AND type = ? and parent_id is NULL", component.SystemTemplateID, component.Name, component.Type)
		} else {
			tx = Gorm.Find(existing, "system_template_id = ? AND name = ? AND type = ? and parent_id = ?", component.SystemTemplateID, component.Name, component.Type, component.ParentId)
		}
	}
	if tx.Error != nil {
		return persisted, tx.Error
	}
	if existing.ID != uuid.Nil {
		component.ID = existing.ID
		tx = Gorm.Table("components").Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "system_template_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).UpdateColumns(component).Update("deleted_at", nil) // explicitly set deleted_at to null; UpdateColumns doesn't set deleted_at to null. Needed in case a component is deleted but found again in the next sync
	} else {
		tx = Gorm.Table("components").Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "system_template_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).Create(component)
	}
	if tx.Error != nil {
		return persisted, tx.Error
	}
	persisted = append(persisted, component.ID)
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
			persisted = append(persisted, childIDs...)
		}
	}
	return persisted, tx.Error
}

func UpdateStatusAndSummaryForComponent(id uuid.UUID, status pkg.ComponentStatus, summary v1.Summary) (int64, error) {
	tx := Gorm.Table("components").Where("id = ? and (status != ? or summary != ?)", id, status, summary).UpdateColumns(pkg.Component{Status: status, Summary: summary})
	return tx.RowsAffected, tx.Error
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
	tx := Gorm.Where("system_template_id = ?", systemTemplateID).Find(componentsModel).UpdateColumn("deleted_at", deleteTime)
	DeleteComponentRelationshipForComponents(componentsModel, deleteTime)
	for _, component := range *componentsModel {
		if component.ComponentChecks != nil {
			if err := DeleteInlineCanariesForComponent(component.ID.String(), deleteTime); err != nil {
				logger.Debugf("Error deleting inline canaries for component %s: %v", component.ID, err)
				continue
			}
		}

		if component.Configs != nil {
			if err := DeleteConfigRelationshipForComponent(component.ID, deleteTime); err != nil {
				logger.Debugf("Error deleting config relationships for component %s: %v", component.ID, err)
				continue
			}
		}
	}
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
	return Gorm.Table("component_relationships").Where("component_id = ? or relationship_id = ?", componentID, componentID).UpdateColumn("deleted_at", deleteTime).Error
}

// DeleteComponentsWithID deletes all components with specified ids.
func DeleteComponentsWithIDs(compIDs []string, deleteTime time.Time) error {
	logger.Infof("deleting component with ids: %v", compIDs)
	tx := Gorm.Table("components").Where("id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime)
	if tx.Error != nil {
		return tx.Error
	}
	tx = Gorm.Table("component_relationships").Where("component_id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime)
	if tx.Error != nil {
		return tx.Error
	}
	if err := Gorm.Table("check_component_relationships").Where("component_id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	for _, compID := range compIDs {
		if err := DeleteInlineCanariesForComponent(compID, deleteTime); err != nil {
			logger.Debugf("Error deleting component relationship for component %v", compID)
		}
	}
	return tx.Error
}

func DeleteInlineCanariesForComponent(componentID string, deleteTime time.Time) error {
	var canaries = []*pkg.Canary{}
	source := "component/" + componentID
	if err := Gorm.Where("source = ?", source).Find(&canaries).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	for _, c := range canaries {
		if err := DeleteChecksForCanary(c.ID.String(), deleteTime); err != nil {
			logger.Debugf("Error deleting checks for canary %v", c.ID)
			continue
		}
		if err := DeleteCheckComponentRelationshipsForCanary(c.ID.String(), deleteTime); err != nil {
			logger.Debugf("Error deleting check component relationships for canary %v", c.ID)
			continue
		}
	}
	return nil
}

func GetActiveComponentsIDsWithSystemTemplateID(systemID string) (compIDs []uuid.UUID, err error) {
	logger.Tracef("Finding components with system id: %s", systemID)
	if err := Gorm.Table("components").Where("deleted_at is NULL and system_template_id = ?", systemID).Select("id").Find(&compIDs).Error; err != nil {
		return nil, err
	}
	return
}
