package checks

import (
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
		canaries, err := db.GetCanariesWithSelectors(*component)
		if err != nil {
			logger.Errorf("error getting canaries with selectors: %s. err: %v", component.ComponentCanaries, err)
			continue
		}
		for _, c := range canaries {
			if err := canaryJobs.SyncCanaryJob(*c.ToV1()); err != nil {
				logger.Debugf("error syncing canary job: %v. Continuing anyway...", err)
			}
			checks, err := db.GetAllChecksForCanary(c.ID)
			if err != nil {
				logger.Debugf("error getting checks for canary: %s. err: %v", c.ID, err)
				continue
			}

			relationships, err := db.GetCheckRelationships(c.ID, component.ID, checks, component.ComponentCanaries)
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
}
