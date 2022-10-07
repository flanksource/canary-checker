package configs

import (
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
)

func ComponentConfigRun() {
	logger.Debugf("Syncing Component Config Relationships")
	components, err := db.GetAllComponentsWithConfigs()
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}

	for _, component := range components {
		if err := db.UpsertComponentConfigRelationship(component.ID, component.Configs); err != nil {
			logger.Errorf("error persisting relationships: %v", err)
			continue
		}
	}
}
