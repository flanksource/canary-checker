package topology

import (
	"fmt"

	"github.com/flanksource/commons/collections"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
)

var ComponentConfigRun = &job.Job{
	Name:       "ComponentConfigRun",
	Schedule:   "@every 2m",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionHour,
	Fn: func(run job.JobRuntime) error {
		db := run.DB().Session(&gorm.Session{NewDB: true})
		var components = []pkg.Component{}
		if err := db.Where(duty.LocalFilter).
			Where("configs != 'null'").
			Select("id", "configs").
			Find(&components).Error; err != nil {
			return fmt.Errorf("error getting components: %v", err)
		}

		for _, component := range components {
			if err := SyncComponentConfigRelationship(db, component); err != nil {
				run.History.AddError(fmt.Sprintf("error persisting config relationships: %v", err))
				continue
			}
			run.History.IncrSuccess()
		}
		return nil
	},
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

func SyncComponentConfigRelationship(db *gorm.DB, component pkg.Component) error {
	if len(component.Configs) == 0 {
		return nil
	}

	relationships, err := component.GetConfigs(db)
	if err != nil {
		return err
	}
	var selectorIDs = lo.Map(relationships, models.ConfigSelectorID)
	var existingConfigIDs = lo.Map(relationships, models.ConfigID)

	var newConfigsIDs []string
	for _, config := range component.Configs {
		dbConfigs, err := query.FindConfigs(db, *config)
		if err != nil {
			return errors.Wrap(err, "error fetching config from database")
		}
		for _, dbConfig := range dbConfigs {
			newConfigsIDs = append(newConfigsIDs, dbConfig.ID.String())

			selectorID := dbConfig.GetSelectorID()
			// If selectorID already exists, no action is required
			if collections.Contains(selectorIDs, selectorID) {
				continue
			}

			// If configID does not exist, create a new relationship
			if !collections.Contains(existingConfigIDs, dbConfig.ID.String()) {
				if err := PersistConfigComponentRelationship(db, dbConfig.ID, component.ID, selectorID); err != nil {
					return errors.Wrap(err, "error persisting config relationships")
				}
				continue
			}

			// If config_id exists mark old row as deleted and update selector_id
			if err := db.Table("config_component_relationships").Where("component_id = ? AND config_id = ?", component.ID, dbConfig.ID).
				Update("deleted_at", duty.Now()).Error; err != nil {
				return errors.Wrap(err, "error updating config relationships")
			}
			if err := PersistConfigComponentRelationship(db, dbConfig.ID, component.ID, selectorID); err != nil {
				return errors.Wrap(err, "error persisting config relationships")
			}
		}
	}

	// Take set difference of these child component Ids and delete them
	configIDsToDelete := utils.SetDifference(existingConfigIDs, newConfigsIDs)
	if len(configIDsToDelete) == 0 {
		return nil
	}
	if err := db.Table("config_component_relationships").Where("component_id = ? AND config_id IN ?", component.ID, configIDsToDelete).
		Update("deleted_at", duty.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale config component relationships")
	}

	return nil
}
