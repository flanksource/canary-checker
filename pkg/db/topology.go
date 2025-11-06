package db

import (
	"fmt"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	dutydb "github.com/flanksource/duty/db"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

const (
	DefaultTopologySchedule = "@every 10m"
)

func PersistV1Topology(ctx context.Context, t *v1.Topology) (pkg.Topology, bool, error) {
	var err error
	var changed bool

	if t.Spec.Schedule == "" {
		t.Spec.Schedule = DefaultTopologySchedule
	}
	model := pkg.TopologyFromV1(t)
	if t.GetPersistedID() != "" {
		model.ID, err = uuid.Parse(t.GetPersistedID())
		if err != nil {
			return model, changed, err
		}
	}
	changed, err = PersistTopology(ctx, &model)
	t.SetUID(k8sTypes.UID(model.ID.String()))
	return model, changed, err
}

func PersistTopology(ctx context.Context, model *pkg.Topology) (bool, error) {
	tx := ctx.DB().
		Clauses(models.Topology{}.OnConflictClause()).
		Create(model)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
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

func GetTopology(ctx context.Context, id string) (*pkg.Topology, error) {
	var t pkg.Topology
	if err := ctx.DB().Table("topologies").Where("id = ? AND deleted_at is NULL", id).First(&t).Error; err != nil {
		return nil, err
	}

	return &t, nil
}

// TODO: Simplify logic and improve readability
func PersistComponent(ctx context.Context, component *pkg.Component) ([]uuid.UUID, error) {
	var existing *models.Component
	var err error
	var persisted []uuid.UUID
	db := ctx.DB()

	existing, err = component.FindExisting(ctx)
	if err != nil {
		return persisted, fmt.Errorf("error finding component: %w", err)
	}

	tx := db.Table("components")
	if existing != nil && existing.ID != uuid.Nil {
		component.ID = existing.ID
		tx = tx.Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "topology_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).UpdateColumns(component)

		if existing.DeletedAt != component.DeletedAt {
			// Since gorm ignores nil fields, we are setting deleted_at explicitly
			if err := db.Table("components").Where("id = ?", existing.ID).UpdateColumn("deleted_at", nil).Error; err != nil {
				return nil, fmt.Errorf("failed to undelete: %w", err)
			}
		}
	} else {
		tx = tx.Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "topology_id"}, {Name: "name"}, {Name: "type"}, {Name: "parent_id"}},
				UpdateAll: true,
			},
		).Create(component)
	}
	if tx.Error != nil {
		return persisted, dutydb.ErrorDetails(tx.Error)
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
			return persisted, fmt.Errorf("error persisting child component[%s] of component[%s]: %w", child.String(), component.ID, dutydb.ErrorDetails(err))
		} else {
			persisted = append(persisted, childIDs...)
		}
	}

	return persisted, dutydb.ErrorDetails(tx.Error)
}

func UpdateStatusAndSummaryForComponent(db *gorm.DB, id uuid.UUID, status types.ComponentStatus, summary types.Summary) (int64, error) {
	tx := db.Table("components").Where("id = ? and (status != ? or summary != ?)", id, status, summary).
		UpdateColumns(models.Component{Status: status, Summary: summary})
	return tx.RowsAffected, tx.Error
}

func DeleteTopology(db *gorm.DB, topologyID string) error {
	if err := db.Table("topologies").Where("id = ?", topologyID).UpdateColumn("deleted_at", duty.Now()).Error; err != nil {
		return fmt.Errorf("error marking topology[%s] as deleted: %w", topologyID, err)
	}
	return DeleteComponentsOfTopology(db, topologyID)
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

// DeleteComponentsWithIDsInBatches deletes all components with specified ids.
func DeleteComponentsWithIDsInBatches(db *gorm.DB, compIDs []string, batchSize int) error {
	batches := lo.Chunk(compIDs, batchSize)
	for _, batch := range batches {
		if err := DeleteComponentsWithIDs(db, batch); err != nil {
			return err
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
	if err := db.Table("components").Where("deleted_at is NULL AND topology_id = ?", topologyID).Select("id").Find(&compIDs).Error; err != nil {
		return nil, err
	}
	return
}
