package checks

import (
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
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
		checks := GetAllChecksForComponentChecks(component)
		relationships, err := db.GetCheckRelationships(component.ID, checks, component.ComponentChecks)
		if err != nil {
			logger.Errorf("error getting relationships: %v", err)
			continue
		}
		err = db.PersisteCheckComponentRelationships(relationships)
		if err != nil {
			logger.Errorf("error persisting relationships: %v", err)
			continue
		}
	}
}

func GetAllChecksForComponentChecks(component *pkg.Component) (checks pkg.Checks) {
	for _, componentCheck := range component.ComponentChecks {
		if componentCheck.Selector.LabelSelector != "" {
			labelChecks, err := db.GetChecksWithLabelSelector(componentCheck.Selector.LabelSelector)
			if err != nil {
				logger.Debugf("error getting checks with label selector: %s. err: %v", componentCheck.Selector.LabelSelector, err)
			}
			checks = append(checks, labelChecks...)
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
			checks = append(checks, inlineChecks...)
		}
	}
	return checks
}
