package checks

import (
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
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
		err = db.PersistCheckComponentRelationshipsForComponent(component.ID, relationships)
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
