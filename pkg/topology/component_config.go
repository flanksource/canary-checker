package topology

import (
	"fmt"

	"github.com/flanksource/commons/collections"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
)

// ComponentConfigsRelationship
type ComponentConfigsRelationship struct {
	ComponentID string
	ConfigIDs   pq.StringArray `gorm:"type:[]text"`
}

var ComponentConfigRun = &job.Job{
	Name:       "ComponentConfigRun",
	Schedule:   "@every 2m",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionFew,
	Fn: func(run job.JobRuntime) error {
		db := run.DB().Session(&gorm.Session{NewDB: true})

		var components []pkg.Component
		if err := db.Where(duty.LocalFilter).
			Select("id", "configs").
			Where("configs != 'null'").
			Find(&components).Error; err != nil {
			return fmt.Errorf("error getting components: %w", err)
		}

		var existingRelationships []ComponentConfigsRelationship
		if err := db.Model(&models.ConfigComponentRelationship{}).
			Select("component_id, ARRAY_AGG(config_id) AS config_ids").
			Where("deleted_at IS NULL").
			Group("component_id").Find(&existingRelationships).Error; err != nil {
			return fmt.Errorf("error fetching existing config_component_relationships: %w", err)
		}

		existingConfigIDsByComponentID := make(map[string][]string)
		for _, er := range existingRelationships {
			existingConfigIDsByComponentID[er.ComponentID] = er.ConfigIDs
		}

		for _, component := range components {
			if err := SyncComponentConfigRelationship(run.Context, component, existingConfigIDsByComponentID[component.ID.String()]); err != nil {
				run.History.AddError(fmt.Sprintf("error persisting config relationships: %v", err))
				continue
			}
			run.History.IncrSuccess()
		}

		// Cleanup dead relationships
		cleanupQuery := `
            UPDATE config_component_relationships
            SET deleted_at = NOW()
            WHERE component_id IN (
                SELECT id FROM components WHERE configs = 'null'
            )
        `
		if err := db.Exec(cleanupQuery).Error; err != nil {
			return fmt.Errorf("error cleaning up old config_component_relationships: %w", err)
		}

		return nil
	},
}

func PersistConfigComponentRelationships(ctx context.Context, rels []models.ConfigComponentRelationship) error {
	if len(rels) == 0 {
		return nil
	}

	return ctx.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "config_id"}, {Name: "component_id"}},
		DoUpdates: clause.Assignments(map[string]any{"deleted_at": nil, "updated_at": duty.Now()}),
	}).Create(&rels).Error
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

func SyncComponentConfigRelationship(ctx context.Context, component pkg.Component, existingConfigIDs []string) error {
	if len(component.Configs) == 0 {
		return nil
	}

	var newConfigsIDs []string
	var relationshipsToPersist []models.ConfigComponentRelationship

	for _, config := range component.Configs {
		dbConfigIDs, err := query.FindConfigIDs(ctx, -1, *config)
		if err != nil {
			return errors.Wrap(err, "error fetching config from database")
		}

		for _, dbConfigID := range dbConfigIDs {
			newConfigsIDs = append(newConfigsIDs, dbConfigID.String())

			if collections.Contains(existingConfigIDs, dbConfigID.String()) {
				continue
			}

			relationshipsToPersist = append(relationshipsToPersist, models.ConfigComponentRelationship{
				ConfigID:    dbConfigID,
				ComponentID: component.ID,
			})
		}
	}

	if err := PersistConfigComponentRelationships(ctx, relationshipsToPersist); err != nil {
		return errors.Wrapf(err, "error persisting config component relationships for component[%s]", component.ID)
	}

	// Take set difference of these child component Ids and delete them
	configIDsToDelete := utils.SetDifference(existingConfigIDs, newConfigsIDs)
	if len(configIDsToDelete) > 0 {
		if err := ctx.DB().Table("config_component_relationships").
			Where("component_id = ? AND config_id IN ?", component.ID, configIDsToDelete).
			Update("deleted_at", duty.Now()).
			Error; err != nil {
			return errors.Wrap(err, "error deleting stale config component relationships")
		}
	}

	return nil
}
