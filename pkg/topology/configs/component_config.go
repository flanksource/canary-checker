package configs

import (
	"time"

	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
)

func ComponentConfigRun() {
	logger.Debugf("Syncing Component Config Relationships")
	components, err := db.GetAllComponentsWithConfigs()
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}

	for _, component := range components {
		jobHistory := models.NewJobHistory("ComponentConfigRelationshipSync", "component", component.ID.String()).Start()
		_ = db.PersistJobHistory(jobHistory)
		if err := SyncComponentConfigRelationship(component.ID, component.Configs); err != nil {
			logger.Errorf("error persisting config relationships: %v", err)
			_ = db.PersistJobHistory(jobHistory.AddError(err.Error()).End())
			continue
		}
		_ = db.PersistJobHistory(jobHistory.IncrSuccess().End())
	}
}

func SyncComponentConfigRelationship(componentID uuid.UUID, configs pkg.Configs) error {
	if len(configs) == 0 {
		return nil
	}

	var selectorIDs []string
	var existingConfigIDs []string
	relationships, err := db.GetConfigRelationshipsForComponent(componentID)
	if err != nil {
		return err
	}

	for _, r := range relationships {
		selectorIDs = append(selectorIDs, r.SelectorID)
		existingConfigIDs = append(existingConfigIDs, r.ConfigID.String())
	}

	var newConfigsIDs []string
	for _, config := range configs {
		dbConfig, err := db.FindConfig(*config)
		if dbConfig == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Tracef("no config found for %s", *config)
			continue
		} else if err != nil {
			return errors.Wrap(err, "error fetching config from database")
		}
		newConfigsIDs = append(newConfigsIDs, dbConfig.ID.String())

		selectorID := dbConfig.GetSelectorID()
		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, selectorID) {
			continue
		}

		// If configID does not exist, create a new relationship
		if !collections.Contains(existingConfigIDs, dbConfig.ID.String()) {
			if err := db.PersistConfigComponentRelationship(dbConfig.ID, componentID, selectorID); err != nil {
				return errors.Wrap(err, "error persisting config relationships")
			}
			continue
		}

		// If config_id exists mark old row as deleted and update selector_id
		if err := db.Gorm.Table("config_component_relationships").Where("component_id = ? AND config_id = ?", componentID, dbConfig.ID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating config relationships")
		}
		if err := db.PersistConfigComponentRelationship(dbConfig.ID, componentID, selectorID); err != nil {
			return errors.Wrap(err, "error persisting config relationships")
		}
	}

	// Take set difference of these child component Ids and delete them
	configIDsToDelete := utils.SetDifference(existingConfigIDs, newConfigsIDs)
	if err := db.Gorm.Table("config_component_relationships").Where("component_id = ? AND config_id IN ?", componentID, configIDsToDelete).
		Update("deleted_at", time.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale config component relationships")
	}

	return nil
}
