package configs

import (
	"fmt"
	"time"

	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/flanksource/canary-checker/pkg/utils"
)

var ComponentConfigRun = &job.Job{
	Name:       "ComponentConfigRun",
	JobHistory: true,
	Schedule:   "@every 2m",
	Singleton:  true,
	Retention: job.Retention{
		Success:  1,
		Failed:   1,
		Age:      time.Hour * 24,
		Interval: time.Hour,
	},
	Fn: func(run job.JobRuntime) error {

		var components = []models.Component{}
		if err := run.DB().
			Where("configs != 'null'").
			Where("agent_id = '00000000-0000-0000-0000-000000000000'").
			Where("deleted_at IS NULL").
			Find(&components).Error; err != nil {
			return fmt.Errorf("error getting components: %v", err)
		}

		for _, component := range components {
			if err := SyncComponentConfigRelationship(run.Context, component.ID, component.Configs); err != nil {
				logger.Errorf("error persisting config relationships: %v", err)
				run.History.AddError(err.Error())
				continue
			}
			run.History.IncrSuccess()
		}
		return nil
	},
}

func GetConfigRelationshipsForComponent(db *gorm.DB, componentID uuid.UUID) ([]models.ConfigComponentRelationship, error) {
	var relationships []models.ConfigComponentRelationship
	if err := db.Where("component_id = ? AND deleted_at IS NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
}

func PersistConfigComponentRelationship(db *gorm.DB, configID, componentID uuid.UUID, selectorID string) error {
	relationship := models.ConfigComponentRelationship{
		ComponentID: componentID,
		ConfigID:    configID,
		SelectorID:  selectorID,
		DeletedAt:   nil,
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "config_id"}, {Name: "component_id"}},
		UpdateAll: true,
	}).Create(&relationship).Error
}

func SyncComponentConfigRelationship(ctx context.Context, componentID uuid.UUID, configs types.ConfigQueries) error {
	if len(configs) == 0 {
		return nil
	}
	db := ctx.DB()

	var selectorIDs []string
	var existingConfigIDs []string
	relationships, err := GetConfigRelationshipsForComponent(db, componentID)
	if err != nil {
		return err
	}

	for _, r := range relationships {
		selectorIDs = append(selectorIDs, r.SelectorID)
		existingConfigIDs = append(existingConfigIDs, r.ConfigID.String())
	}

	var newConfigsIDs []string
	for _, config := range configs {
		dbConfig, err := query.FindConfig(ctx, *config)
		if dbConfig == nil || errors.Is(err, gorm.ErrRecordNotFound) && ctx.IsDebug() {
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
			if err := PersistConfigComponentRelationship(db, dbConfig.ID, componentID, selectorID); err != nil {
				return errors.Wrap(err, "error persisting config relationships")
			}
			continue
		}

		// If config_id exists mark old row as deleted and update selector_id
		if err := db.Table("config_component_relationships").Where("component_id = ? AND config_id = ?", componentID, dbConfig.ID).
			Update("deleted_at", duty.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating config relationships")
		}
		if err := PersistConfigComponentRelationship(db, dbConfig.ID, componentID, selectorID); err != nil {
			return errors.Wrap(err, "error persisting config relationships")
		}
	}

	// Take set difference of these child component Ids and delete them
	configIDsToDelete := utils.SetDifference(existingConfigIDs, newConfigsIDs)
	if err := db.Table("config_component_relationships").Where("component_id = ? AND config_id IN ?", componentID, configIDsToDelete).
		Update("deleted_at", duty.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale config component relationships")
	}

	return nil
}
