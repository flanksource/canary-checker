package configs

import (
	"time"

	"github.com/flanksource/commons/collections"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/flanksource/canary-checker/pkg"
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
		if err := SyncComponentConfigRelationship(component.ID, component.Configs); err != nil {
			logger.Errorf("error persisting relationships: %v", err)
			continue
		}
	}
}

func SyncComponentConfigRelationship(componentID uuid.UUID, configs pkg.Configs) error {
	if len(configs) == 0 {
		return nil
	}

	var selectorIDs []string
	var configIDs []string
	relationships, err := db.GetConfigRelationshipsForComponent(componentID)
	if err != nil {
		return err
	}

	for _, r := range relationships {
		selectorIDs = append(selectorIDs, r.SelectorID)
		configIDs = append(configIDs, r.ConfigID.String())
	}

	for _, config := range configs {
		dbConfig, err := db.FindConfig(*config)
		if err != nil {
			return errors.Wrap(err, "error fetching config from database")
		}
		selectorID := dbConfig.GetSelectorID()

		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, selectorID) {
			continue
		}

		// If configID does not exist, create a new relationship
		if !collections.Contains(configIDs, dbConfig.ID.String()) {
			if err := db.PersistConfigComponentRelationship(dbConfig.ID, componentID, selectorID); err != nil {
				return errors.Wrap(err, "error persisting config relationships")
			}
			continue
		}

		// If config_id exists mark old row as deleted and update selector_id
		if err := db.Gorm.Model(&db.ConfigComponentRelationship{}).Where("component_id = ? AND config_id = ?", componentID, dbConfig.ID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating config relationships")
		}
		if err := db.PersistConfigComponentRelationship(config.ID, componentID, selectorID); err != nil {
			return errors.Wrap(err, "error persisting config relationships")
		}
	}
	return nil
}
