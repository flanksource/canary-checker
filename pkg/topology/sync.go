package topology

import (
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Fetches and updates the selected component for components
func ComponentRun() {
	logger.Debugf("Syncing Component Relationships")

	components, err := db.GetAllComponentWithSelectors()
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}

	for _, component := range components {
		comps, err := db.GetComponentsWithSelectors(component.Selectors)
		if err != nil {
			logger.Errorf("error getting components with selectors: %s. err: %v", component.Selectors, err)
			continue
		}
		relationships, err := db.NewComponentRelationships(component.ID, component.Path, comps)
		if err != nil {
			logger.Errorf("error getting relationships: %v", err)
			continue
		}
		err = SyncComponentRelationships(component.ID, relationships)
		if err != nil {
			logger.Errorf("error syncing relationships: %v", err)
			continue
		}

		// // Sync config relationships
		// if err := db.UpsertComponentConfigRelationship(component.ID, component.Configs); err != nil {
		// 	logger.Errorf("error upserting config relationships: %v", err)
		// }
	}
}

func ComponentStatusSummarySync() {
	logger.Debugf("Syncing Status and Summary for components")
	components, err := Query(TopologyParams{
		Depth: 0,
	})
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}
	for _, component := range components.Walk() {
		_, err = db.UpdateStatusAndSummaryForComponent(component.ID, component.Status, component.Summary)
		if err != nil {
			logger.Errorf("error persisting component: %v", err)
			continue
		}
	}
}

func SyncComponentRelationships(parentComponentID uuid.UUID, relationships []*pkg.ComponentRelationship) error {
	var selectorIDs, childComponentIDs []string

	existingRelationShips, err := db.GetChildRelationshipsForParentComponent(parentComponentID)
	if err != nil {
		return err
	}
	for _, r := range existingRelationShips {
		selectorIDs = append(selectorIDs, r.SelectorID)
		childComponentIDs = append(childComponentIDs, r.ComponentID.String())
	}

	var newChildComponentIDs []string
	for _, r := range relationships {
		newChildComponentIDs = append(newChildComponentIDs, r.ComponentID.String())

		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, r.SelectorID) {
			continue
		}

		// If childComponentID does not exist, create a new relationship
		if !collections.Contains(childComponentIDs, r.ComponentID.String()) {
			if err := db.PersistComponentRelationship(r); err != nil {
				return errors.Wrap(err, "error persisting component relationships")
			}
			continue
		}

		// If childComponentID exists mark old row as deleted and update selector_id
		if err := db.Gorm.Table("component_relationships").Where("relationship_id = ? AND component_id = ?", parentComponentID, r.ComponentID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating component relationships")
		}

		if err := db.PersistComponentRelationship(r); err != nil {
			return err
		}
	}

	// Take set difference of these child component Ids and delete them
	childComponentIDsToDelete := utils.SetDifference(childComponentIDs, newChildComponentIDs)
	if err := db.Gorm.Table("component_relationships").Where("relationship_id = ? AND component_id IN ?", parentComponentID, childComponentIDsToDelete).
		Update("deleted_at", time.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale component relationships")
	}

	return nil
}

func ComponentCostRun() {
	logger.Debugf("Syncing component costs")

	componentConfigIDs, err := db.GetAllComponentsWithConfigRelationships()
	if err != nil {
		logger.Errorf("Error getting components: %v", err)
		return
	}

	for componentID, configIDs := range componentConfigIDs {
		err = db.UpdateComponentCosts(componentID, configIDs)
		if err != nil {
			logger.Errorf("Error updating component costs: %v", err)
		}
	}
}
