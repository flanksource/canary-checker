package checks

import (
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func ComponentCheckRun() {
	logger.Debugf("Syncing Check Relationships")
	components, err := db.GetAllComponentWithCanaries()
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}
	for _, component := range components {
		relationships := GetCheckComponentRelationshipsForComponent(component)
		err = SyncCheckComponentRelationshipsForComponent(component.ID, relationships)
		if err != nil {
			logger.Errorf("error persisting relationships: %v", err)
			continue
		}
	}
}

func GetCheckComponentRelationshipsForComponent(component *pkg.Component) (relationships []*pkg.CheckComponentRelationship) {
	for _, componentCheck := range component.ComponentChecks {
		if componentCheck.Selector.LabelSelector != "" {
			labelChecks, err := db.GetChecksWithLabelSelector(componentCheck.Selector.LabelSelector)
			if err != nil {
				logger.Debugf("error getting checks with label selector: %s. err: %v", componentCheck.Selector.LabelSelector, err)
			}
			for _, labelCheck := range labelChecks {
				selectorID, err := utils.GenerateJSONMD5Hash(componentCheck)
				if err != nil {
					logger.Errorf("Error generationg selector_id hash: %v", err)
				}

				relationships = append(relationships, &pkg.CheckComponentRelationship{
					CanaryID:    labelCheck.CanaryID,
					CheckID:     labelCheck.ID,
					ComponentID: component.ID,
					SelectorID:  selectorID,
				})
			}
		}
		if componentCheck.Inline != nil {
			canary, err := db.CreateComponentCanaryFromInline(component.ID.String(), component.Name, component.Namespace, component.Schedule, component.Owner, componentCheck.Inline)
			if err != nil {
				logger.Debugf("error creating canary from inline: %v", err)
			}
			if err := canaryJobs.SyncCanaryJob(*canary.ToV1()); err != nil {
				logger.Debugf("error creating canary job: %v", err)
			}
			inlineChecks, err := db.GetAllChecksForCanary(canary.ID)
			if err != nil {
				logger.Debugf("error getting checks for canary: %s. err: %v", canary.ID, err)
				continue
			}
			for _, inlineCheck := range inlineChecks {
				selectorID, err := utils.GenerateJSONMD5Hash(componentCheck)
				if err != nil {
					logger.Errorf("Error generationg selector_id hash: %v", err)
				}

				relationships = append(relationships, &pkg.CheckComponentRelationship{
					CanaryID:    inlineCheck.CanaryID,
					CheckID:     inlineCheck.ID,
					ComponentID: component.ID,
					SelectorID:  selectorID,
				})
			}
		}
	}
	return relationships
}

func SyncCheckComponentRelationshipsForComponent(componentID uuid.UUID, relationships []*pkg.CheckComponentRelationship) error {
	var selectorIDs, checkIDs []string
	existingRelationShips, err := db.GetCheckRelationshipsForComponent(componentID)
	if err != nil {
		return err
	}
	for _, r := range existingRelationShips {
		selectorIDs = append(selectorIDs, r.SelectorID)
		checkIDs = append(checkIDs, r.CheckID.String())
	}

	var newCheckIDs []string
	for _, r := range relationships {
		newCheckIDs = append(newCheckIDs, r.CheckID.String())

		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, r.SelectorID) {
			continue
		}

		// If checkID does not exist, create a new relationship
		if !collections.Contains(checkIDs, r.CheckID.String()) {
			if err := db.PersistCheckComponentRelationship(r); err != nil {
				return errors.Wrap(err, "error persisting check component relationships")
			}
			continue
		}

		// If check_id exists mark old row as deleted and update selector_id
		if err := db.Gorm.Table("check_component_relationships").Where("component_id = ? AND check_id = ?", componentID, r.CheckID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating check relationships")
		}

		if err := db.PersistCheckComponentRelationship(r); err != nil {
			return errors.Wrap(err, "error persisting check component relationships")
		}
	}

	// Take set difference of these child component Ids and delete them
	checkIDsToDelete := utils.SetDifference(checkIDs, newCheckIDs)
	if err := db.Gorm.Table("check_component_relationships").Where("component_id = ? AND check_id IN ?", componentID, checkIDsToDelete).
		Update("deleted_at", time.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale check component relationships")
	}

	return nil
}
