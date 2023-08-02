package topology

import (
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Fetches and updates the selected component for components
func ComponentRun() {
	logger.Debugf("Syncing Component Relationships")

	components, err := db.GetAllComponentsWithSelectors()
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}

	jobHistory := models.NewJobHistory("ComponentRelationshipSync", "", "").Start()
	_ = db.PersistJobHistory(jobHistory)
	for _, component := range components {
		comps, err := db.GetComponentsWithSelectors(component.Selectors)
		if err != nil {
			logger.Errorf("error getting components with selectors: %s. err: %v", component.Selectors, err)
			jobHistory.AddError(err.Error())
			continue
		}
		relationships, err := db.NewComponentRelationships(component.ID, component.Path, comps)
		if err != nil {
			logger.Errorf("error getting relationships: %v", err)
			jobHistory.AddError(err.Error())
			continue
		}
		err = SyncComponentRelationships(component.ID, relationships)
		if err != nil {
			logger.Errorf("error syncing relationships: %v", err)
			jobHistory.AddError(err.Error())
			continue
		}
		jobHistory.IncrSuccess()
	}
	_ = db.PersistJobHistory(jobHistory.End())
}

func ComponentStatusSummarySync() {
	logger.Debugf("Syncing Status and Summary for components")
	topology, err := Query(duty.TopologyOptions{Depth: 3})
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}
	jobHistory := models.NewJobHistory("ComponentStatusSummarySync", "", "").Start()
	_ = db.PersistJobHistory(jobHistory)
	topology.Components.Map(func(c *models.Component) {
		if _, err := db.UpdateStatusAndSummaryForComponent(c.ID, c.Status, c.Summary); err != nil {
			logger.Errorf("error persisting component: %v", err)
			jobHistory.AddError(err.Error())
		}
		jobHistory.IncrSuccess()
	})
	_ = db.PersistJobHistory(jobHistory.End())
}

func SyncComponentRelationships(parentComponentID uuid.UUID, relationships []*pkg.ComponentRelationship) error {
	existingRelationships, err := db.GetChildRelationshipsForParentComponent(parentComponentID)
	if err != nil {
		return err
	}

	var childComponentIDs []string
	for _, r := range existingRelationships {
		childComponentIDs = append(childComponentIDs, r.ComponentID.String())
	}

	var newChildComponentIDs []string
	for _, r := range relationships {
		newChildComponentIDs = append(newChildComponentIDs, r.ComponentID.String())
	}
	if err := db.PersistComponentRelationships(relationships); err != nil {
		return err
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

	jobHistory := models.NewJobHistory("ComponentCostSync", "", "").Start()
	err := db.UpdateComponentCosts()
	if err != nil {
		logger.Errorf("Error updating component costs: %v", err)
		_ = db.PersistJobHistory(jobHistory.AddError(err.Error()).End())
		return
	}
	_ = db.PersistJobHistory(jobHistory.IncrSuccess().End())
}
