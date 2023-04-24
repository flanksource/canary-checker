package db

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func PersistTopology(t *v1.Topology) (string, bool, error) {
	model := pkg.TopologyFromV1(t)
	if t.GetPersistedID() != "" {
		model.ID, _ = uuid.Parse(t.GetPersistedID())
	}
	var changed bool
	tx := Gorm.Table("topologies").Clauses(clause.OnConflict{
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

func GetTopology(ctx context.Context, id string) (*v1.Topology, error) {
	var t pkg.Topology
	if err := Gorm.WithContext(ctx).Table("topologies").Where("id = ? AND deleted_at is NULL", id).First(&t).Error; err != nil {
		return nil, err
	}

	tv1 := t.ToV1()
	return &tv1, nil
}

func GetAllTopologies() ([]v1.Topology, error) {
	var v1topologies []v1.Topology
	var topologies []pkg.Topology
	if err := Gorm.Table("topologies").Find(&topologies).Where("deleted_at is NULL").Error; err != nil {
		return nil, err
	}
	for _, t := range topologies {
		v1topologies = append(v1topologies, t.ToV1())
	}
	return v1topologies, nil
}

// Get all the components from table which has not null selectors
func GetAllComponentsWithSelectors() (components pkg.Components, err error) {
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
			labelComponents, err := GetComponentsWithLabelSelector(resourceSelector.LabelSelector)
			if err != nil {
				continue
			}

			for _, c := range labelComponents {
				c.SelectorID = selectorID
				uniqueComponents[c.ID.String()] = c
			}
		}
		if resourceSelector.FieldSelector != "" {
			fieldComponents, err := GetComponentsWithFieldSelector(resourceSelector.FieldSelector)
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

// TODO: Simplify logic and improve readability
func PersistComponent(component *pkg.Component) ([]uuid.UUID, error) {
	existing := &pkg.Component{}
	var persisted []uuid.UUID
	var tx *gorm.DB
	if component.TopologyID == nil {
		if component.ParentId == nil {
			tx = Gorm.Find(existing, "name = ? AND type = ? and parent_id is NULL", component.Name, component.Type)
		} else {
			tx = Gorm.Find(existing, "name = ? AND type = ? and parent_id = ?", component.Name, component.Type, component.ParentId)
		}
	} else {
		if component.ParentId == nil {
			tx = Gorm.Find(existing, "topology_id = ? AND name = ? AND type = ? and parent_id is NULL", component.TopologyID, component.Name, component.Type)
		} else {
			tx = Gorm.Find(existing, "topology_id = ? AND name = ? AND type = ? and parent_id = ?", component.TopologyID, component.Name, component.Type, component.ParentId)
		}
	}
	if tx.Error != nil {
		return persisted, fmt.Errorf("error finding component: %v", tx.Error)
	}

	if existing.ID != uuid.Nil {
		component.ID = existing.ID
		tx = Gorm.Table("components").Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "topology_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).UpdateColumns(component)

		// Since gorm ignores nil fields, we are setting deleted_at explicitly
		Gorm.Table("components").Where("id = ?", existing.ID).UpdateColumn("deleted_at", nil)
	} else {
		tx = Gorm.Table("components").Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "topology_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).Create(component)
	}
	if tx.Error != nil {
		return persisted, tx.Error
	}

	persisted = append(persisted, component.ID)
	for _, child := range component.Components {
		child.TopologyID = component.TopologyID
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

func UpdateStatusAndSummaryForComponent(id uuid.UUID, status models.ComponentStatus, summary models.Summary) (int64, error) {
	tx := Gorm.Table("components").Where("id = ? and (status != ? or summary != ?)", id, status, summary).UpdateColumns(models.Component{Status: status, Summary: summary})
	return tx.RowsAffected, tx.Error
}

func DeleteTopology(t *v1.Topology) error {
	logger.Infof("Deleting topology %s/%s", t.Namespace, t.Name)
	model := pkg.TopologyFromV1(t)
	deleteTime := time.Now()
	if t.GetPersistedID() == "" {
		logger.Errorf("Topology %s/%s has not been persisted", t.Namespace, t.Name)
		return nil
	}
	tx := Gorm.Table("topologies").Find(model, "id = ?", t.GetPersistedID()).UpdateColumn("deleted_at", deleteTime)
	if tx.Error != nil {
		return tx.Error
	}
	return DeleteComponentsOfTopology(t.GetPersistedID(), deleteTime)
}

// DeleteComponents deletes all components associated with a topology
func DeleteComponentsOfTopology(topologyID string, deleteTime time.Time) error {
	logger.Infof("Deleting all components associated with topology: %s", topologyID)
	componentsModel := &[]pkg.Component{}
	if err := Gorm.Where("topology_id = ?", topologyID).Find(componentsModel).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	for _, component := range *componentsModel {
		if err := DeleteComponentChildren(component.ID.String(), deleteTime); err != nil {
			logger.Errorf("Error deleting component[%s] children: %v", component.ID, err)
		}

		if err := DeleteComponentRelationship(component.ID.String(), deleteTime); err != nil {
			logger.Errorf("Error deleting component[%s] relationship for component %v", component.ID, err)
		}

		if component.ComponentChecks != nil {
			if err := DeleteInlineCanariesForComponent(component.ID.String(), deleteTime); err != nil {
				logger.Errorf("Error deleting inline canaries for component %s: %v", component.ID, err)
			}
		}

		if component.Configs != nil {
			if err := DeleteConfigRelationshipForComponent(component.ID, deleteTime); err != nil {
				logger.Errorf("Error deleting config relationships for component %s: %v", component.ID, err)
			}
		}
	}
	return nil
}

func DeleteComponentRelationship(componentID string, deleteTime time.Time) error {
	return Gorm.Table("component_relationships").Where("component_id = ? or relationship_id = ?", componentID, componentID).UpdateColumn("deleted_at", deleteTime).Error
}

// DeleteComponentsWithID deletes all components with specified ids.
func DeleteComponentsWithIDs(compIDs []string, deleteTime time.Time) error {
	logger.Infof("Deleting component ids: %v", compIDs)
	if err := Gorm.Table("components").Where("id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	if err := Gorm.Table("component_relationships").Where("component_id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	if err := Gorm.Table("check_component_relationships").Where("component_id in (?)", compIDs).UpdateColumn("deleted_at", deleteTime).Error; err != nil {
		return err
	}
	for _, compID := range compIDs {
		if err := DeleteInlineCanariesForComponent(compID, deleteTime); err != nil {
			logger.Errorf("Error deleting component[%s] relationship: %v", compID, err)
		}

		if err := DeleteComponentChildren(compID, deleteTime); err != nil {
			logger.Errorf("Error deleting component[%s] children: %v", compID, err)
		}
	}
	return nil
}

func DeleteComponentChildren(componentID string, deleteTime time.Time) error {
	return Gorm.Table("components").
		Where("path LIKE ?", "%"+componentID+"%").
		Update("deleted_at", deleteTime).
		Error
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

func GetActiveComponentsIDsOfTopology(topologyID string) (compIDs []uuid.UUID, err error) {
	logger.Tracef("Finding components with topology id: %s", topologyID)
	if err := Gorm.Table("components").Where("deleted_at is NULL and topology_id = ?", topologyID).Select("id").Find(&compIDs).Error; err != nil {
		return nil, err
	}
	return
}
