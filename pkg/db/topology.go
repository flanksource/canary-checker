package db

import (
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
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

func UpdateComponentCosts() error {
	return Gorm.Exec(`
    WITH 
    component_children AS (
        SELECT components.id, ARRAY(
            SELECT child_id FROM lookup_component_children(components.id::text, -1)
            UNION
            SELECT relationship_id as child_id FROM component_relationships WHERE component_id IN (
                SELECT child_id FROM lookup_component_children(components.id::text, -1)
            )
        ) AS child_ids
        FROM components
        GROUP BY components.id
    ),
    component_configs AS (
        SELECT component_children.id, ARRAY_AGG(ccr.config_id) as config_ids
        FROM component_children
        INNER JOIN config_component_relationships ccr ON ccr.component_id = ANY(component_children.child_ids)
        GROUP BY component_children.id
    ),
    component_config_costs AS (
        SELECT 
            component_configs.id,
            SUM(cost_per_minute) AS cost_per_minute,
            SUM(cost_total_1d) AS cost_total_1d,
            SUM(cost_total_7d) AS cost_total_7d,
            SUM(cost_total_30d) AS cost_total_30d
        FROM config_items
        INNER JOIN component_configs ON config_items.id = ANY(component_configs.config_ids)
        GROUP BY component_configs.id
    )

    UPDATE components
    SET
        cost_per_minute = component_config_costs.cost_per_minute,
        cost_total_1d = component_config_costs.cost_total_1d,
        cost_total_7d = component_config_costs.cost_total_7d,
        cost_total_30d = component_config_costs.cost_total_30d
    FROM component_config_costs
    WHERE components.id = component_config_costs.id
    `).Error
}

func GetComponentsWithSelectors(resourceSelectors v1.ResourceSelectors) (components pkg.Components, err error) {
	var uniqueComponents = make(map[string]*pkg.Component)
	for _, resourceSelector := range resourceSelectors {
		var selectorID string
		selectorID, err = utils.GenerateJSONMD5Hash(resourceSelector)
		if err != nil {
			return components, errors.Wrap(err, fmt.Sprintf("error generating selector_id for resourceSelector: %v", resourceSelector))
		}

		if resourceSelector.LabelSelector != "" {
			labelComponents, err := GetComponensWithLabelSelector(resourceSelector.LabelSelector)
			if err != nil {
				continue
			}

			for _, c := range labelComponents {
				c.SelectorID = selectorID
				uniqueComponents[c.ID.String()] = c
			}
		}
		if resourceSelector.FieldSelector != "" {
			fieldComponents, err := GetComponensWithFieldSelector(resourceSelector.FieldSelector)
			if err != nil {
				continue
			}
			for _, c := range fieldComponents {
				c.SelectorID = selectorID
				uniqueComponents[c.ID.String()] = c
			}
		}
	}
	for _, comp := range uniqueComponents {
		components = append(components, comp)
	}
	return components, nil
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
		relationships = append(relationships, &pkg.ComponentRelationship{
			RelationshipID:   relationshipID,
			ComponentID:      component.ID,
			SelectorID:       component.SelectorID,
			RelationshipPath: path + "." + relationshipID.String(),
		})
	}
	return
}

func GetChildRelationshipsForParentComponent(componentID uuid.UUID) ([]pkg.ComponentRelationship, error) {
	var relationships []pkg.ComponentRelationship
	if err := Gorm.Table("component_relationships").Where("relationship_id = ? AND deleted_at IS NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
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
	if err := Gorm.Where("component_id = ? AND deleted_at IS NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
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
