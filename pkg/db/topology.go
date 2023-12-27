package db

import (
	"fmt"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func PersistTopology(ctx context.Context, t *v1.Topology) (bool, error) {
	var err error
	var changed bool

	model := pkg.TopologyFromV1(t)
	model.ID, err = uuid.Parse(t.GetPersistedID())
	if err != nil {
		return changed, err
	}
	tx := ctx.DB().Table("topologies").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}, {Name: "name"}, {Name: "namespace"}},
		UpdateAll: true,
	}).Create(model)
	if tx.Error != nil {
		return changed, tx.Error
	}
	if tx.RowsAffected > 0 {
		changed = true
	}
	return changed, nil
}

func PersistComponents(ctx context.Context, results []*pkg.Component) error {
	for _, component := range results {
		_, err := PersistComponent(ctx, component)
		if err != nil {
			logger.Errorf("Error persisting component %v", err)
			continue
		}
	}
	return nil
}

func GetTopology(ctx context.Context, id string) (*v1.Topology, error) {
	var t pkg.Topology
	if err := ctx.DB().Table("topologies").Where("id = ? AND deleted_at is NULL", id).First(&t).Error; err != nil {
		return nil, err
	}

	tv1 := t.ToV1()
	return &tv1, nil
}

// TODO: Simplify logic and improve readability
func PersistComponent(ctx context.Context, component *pkg.Component) ([]uuid.UUID, error) {
	existing := models.Component{}
	var persisted []uuid.UUID
	db := ctx.DB()
	var tx *gorm.DB
	if component.ID == uuid.Nil {
		if component.ParentId == nil {
			tx = db.Find(existing, "name = ? AND type = ? and parent_id is NULL", component.Name, component.Type)
		} else {
			tx = db.Find(existing, "name = ? AND type = ? and parent_id = ?", component.Name, component.Type, component.ParentId)
		}
	} else {
		if component.ParentId == nil {
			tx = db.Find(existing, "topology_id = ? AND name = ? AND type = ? and parent_id is NULL", component.TopologyID, component.Name, component.Type)
		} else {
			tx = db.Find(existing, "topology_id = ? AND name = ? AND type = ? and parent_id = ?", component.TopologyID, component.Name, component.Type, component.ParentId)
		}
	}
	if tx.Error != nil {
		return persisted, fmt.Errorf("error finding component: %v", tx.Error)
	}

	if existing.ID != uuid.Nil {
		component.ID = existing.ID
		tx = db.Table("components").Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "topology_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).UpdateColumns(component)

		// Since gorm ignores nil fields, we are setting deleted_at explicitly
		db.Table("components").Where("id = ?", existing.ID).UpdateColumn("deleted_at", nil)
	} else {
		tx = db.Table("components").Clauses(
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
		if childIDs, err := PersistComponent(ctx, child); err != nil {
			logger.Errorf("Error persisting child component of %v, :v", component.ID, err)
		} else {
			persisted = append(persisted, childIDs...)
		}
	}

	return persisted, tx.Error
}

func UpdateStatusAndSummaryForComponent(db *gorm.DB, id uuid.UUID, status types.ComponentStatus, summary types.Summary) (int64, error) {
	tx := db.Table("components").Where("id = ? and (status != ? or summary != ?)", id, status, summary).
		UpdateColumns(models.Component{Status: status, Summary: summary})
	return tx.RowsAffected, tx.Error
}

func DeleteTopology(db *gorm.DB, t *v1.Topology) error {
	logger.Infof("Deleting topology %s/%s", t.Namespace, t.Name)
	model := pkg.TopologyFromV1(t)

	tx := db.Table("topologies").Find(model, "id = ?", t.GetPersistedID()).UpdateColumn("deleted_at", duty.Now())
	if tx.Error != nil {
		return tx.Error
	}
	return DeleteComponentsOfTopology(db, t.GetPersistedID())
}

// DeleteComponents deletes all components associated with a topology
func DeleteComponentsOfTopology(db *gorm.DB, topologyID string) error {
	logger.Infof("Deleting all components associated with topology: %s", topologyID)
	componentsModel := &[]pkg.Component{}
	if err := db.Where("topology_id = ?", topologyID).Find(componentsModel).Error; err != nil {
		return fmt.Errorf("error querying components: %w", err)
	}
	for _, component := range *componentsModel {
		if err := db.Table("components").
			Where("id = ?", component.ID.String()).
			UpdateColumn("deleted_at", duty.Now()).Error; err != nil {
			return fmt.Errorf("error updating deleted_at for components: %w", err)
		}
		if err := DeleteComponentChildren(db, component.ID.String()); err != nil {
			logger.Errorf("Error deleting component[%s] children: %v", component.ID, err)
		}

		if err := DeleteComponentRelationship(db, component.ID.String()); err != nil {
			logger.Errorf("Error deleting component[%s] relationship for component %v", component.ID, err)
		}

		if component.ComponentChecks != nil {
			if err := DeleteInlineCanariesForComponent(db, component.ID.String()); err != nil {
				logger.Errorf("Error deleting inline canaries for component %s: %v", component.ID, err)
			}
		}

		if component.Configs != nil {
			if err := db.Model(&models.ConfigComponentRelationship{}).Where("component_id = ?", component.ID).Update("deleted_at", duty.Now()).Error; err != nil {
				logger.Errorf("Error deleting config relationships for component %s: %v", component.ID, err)
			}
		}
	}
	return nil
}

func DeleteComponentRelationship(db *gorm.DB, componentID string) error {
	return db.Table("component_relationships").Where("component_id = ? or relationship_id = ?", componentID, componentID).UpdateColumn("deleted_at", duty.Now()).Error
}

// DeleteComponentsWithID deletes all components with specified ids.
func DeleteComponentsWithIDs(db *gorm.DB, compIDs []string) error {
	if err := db.Table("components").Where("id in (?)", compIDs).UpdateColumn("deleted_at", duty.Now()).Error; err != nil {
		return err
	}
	if err := db.Table("component_relationships").Where("component_id in (?)", compIDs).UpdateColumn("deleted_at", duty.Now()).Error; err != nil {
		return err
	}
	if err := db.Table("check_component_relationships").Where("component_id in (?)", compIDs).UpdateColumn("deleted_at", duty.Now()).Error; err != nil {
		return err
	}
	for _, compID := range compIDs {
		if err := DeleteInlineCanariesForComponent(db, compID); err != nil {
			logger.Errorf("Error deleting component[%s] relationship: %v", compID, err)
		}

		if err := DeleteComponentChildren(db, compID); err != nil {
			logger.Errorf("Error deleting component[%s] children: %v", compID, err)
		}
	}
	return nil
}

func DeleteComponentChildren(db *gorm.DB, componentID string) error {
	return db.Table("components").
		Where("path LIKE ?", "%"+componentID+"%").
		Update("deleted_at", duty.Now()).
		Error
}

func DeleteInlineCanariesForComponent(db *gorm.DB, componentID string) error {
	var rows []struct {
		ID string
	}
	source := "component/" + componentID
	if err := db.
		Model(&rows).
		Table("canaries").
		Where("source = ?", source).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		UpdateColumn("deleted_at", duty.Now()).Error; err != nil {
		return err
	}

	for _, r := range rows {
		if _, err := DeleteChecksForCanary(db, r.ID); err != nil {
			logger.Errorf("Error deleting checks for canary[%s]: %v", r.ID, err)
		}
		if err := DeleteCheckComponentRelationshipsForCanary(db, r.ID); err != nil {
			logger.Errorf("Error deleting check component relationships for canary[%s]: %v", r.ID, err)
		}
	}
	return nil
}

func GetActiveComponentsIDsOfTopology(db *gorm.DB, topologyID string) (compIDs []uuid.UUID, err error) {
	if err := db.Table("components").Where("deleted_at is NULL and topology_id = ?", topologyID).Select("id").Find(&compIDs).Error; err != nil {
		return nil, err
	}
	return
}
