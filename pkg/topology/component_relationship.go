package topology

import (
	"fmt"

	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gorm.io/gorm/clause"
)

var ComponentRelationshipSync = &job.Job{
	Name:       "ComponentRelationshipSync",
	Schedule:   "@every 5m",
	JobHistory: true,
	Retention:  job.RetentionFew,
	Singleton:  true,
	Fn: func(ctx job.JobRuntime) error {
		var components []models.Component
		if err := ctx.DB().Where(duty.LocalFilter).
			Where("selectors != 'null'").
			Find(&components).Error; err != nil {
			return fmt.Errorf("error getting components: %v", err)
		}

		for _, component := range components {
			hash := component.Selectors.Hash()
			comps, err := query.FindComponents(ctx.Context, ctx.Properties().Int("resource.lookup.limit", 1000), component.Selectors...)
			if err != nil {
				ctx.History.AddError(fmt.Sprintf("error getting components with selectors: %v. err: %v", component.Selectors, err))
				continue
			}
			relationships := []models.ComponentRelationship{}
			for _, c := range comps {
				relationships = append(relationships, models.ComponentRelationship{
					RelationshipID:   component.ID,
					ComponentID:      c.ID,
					SelectorID:       hash,
					RelationshipPath: component.Path + "." + component.ID.String(),
				})
			}

			err = syncComponentRelationships(ctx.Context, component.ID, relationships)
			if err != nil {
				ctx.History.AddError(fmt.Sprintf("error syncing relationships: %v", err))
				continue
			}
			ctx.History.IncrSuccess()
		}

		// Cleanup dead relationships
		cleanupQuery := `
            UPDATE component_relationships
            SET deleted_at = NOW()
            WHERE relationship_id IN (
                SELECT id FROM components WHERE selectors = 'null'
            )
        `
		if err := ctx.DB().Exec(cleanupQuery).Error; err != nil {
			return fmt.Errorf("error cleaning up dead component_relationships: %w", err)
		}

		return nil
	},
}

var ComponentStatusSummarySync = &job.Job{
	Name:       "ComponentStatusSummarySync",
	Schedule:   "@every 2m",
	JobHistory: true,
	Retention:  job.RetentionFew,
	Singleton:  true,
	Fn: func(ctx job.JobRuntime) error {
		topology, err := Query(ctx.Context, duty.TopologyOptions{Depth: 3})
		if err != nil {
			return fmt.Errorf("error getting components: %v", err)
		}

		for _, c := range topology.Components {
			tx := ctx.DB().Where("id = ? and (status != ? or summary != ?)", c.ID, c.Status, c.Summary).
				UpdateColumns(models.Component{Status: c.Status, Summary: c.Summary})
			if tx.Error != nil {
				ctx.History.AddError(tx.Error.Error())
			} else {
				ctx.History.IncrSuccess()
			}
		}

		return nil
	},
}

func syncComponentRelationships(ctx context.Context, id uuid.UUID, relationships []models.ComponentRelationship) error {
	var existingChildComponentIDs []string
	if err := ctx.DB().Table("component_relationships").Select("component_id").Where("relationship_id = ? AND deleted_at IS NULL", id).Find(&existingChildComponentIDs).Error; err != nil {
		return err
	}

	newChildComponentIDs := lo.Map(relationships, func(c models.ComponentRelationship, _ int) string { return c.ComponentID.String() })

	// Take set difference of these child component Ids and delete them
	childComponentIDsToDelete, childComponentIDsToAdd := lo.Difference(existingChildComponentIDs, newChildComponentIDs)

	relationshipsToAdd := lo.Filter(relationships, func(r models.ComponentRelationship, _ int) bool {
		return lo.Contains(childComponentIDsToAdd, r.ComponentID.String())
	})

	if len(relationshipsToAdd) > 0 {
		if err := ctx.DB().Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "component_id"}, {Name: "relationship_id"}, {Name: "selector_id"}},
			DoUpdates: clause.Assignments(map[string]any{"deleted_at": nil}),
		}).Create(relationshipsToAdd).Error; err != nil {
			return err
		}
	}

	if len(childComponentIDsToDelete) > 0 {
		if err := ctx.DB().
			Table("component_relationships").
			Where("relationship_id = ? AND component_id IN ?", id, childComponentIDsToDelete).
			Update("deleted_at", duty.Now()).
			Error; err != nil {
			return errors.Wrap(err, "error deleting stale component relationships")
		}
	}

	return nil
}

var ComponentCostRun = &job.Job{
	Name:       "ComponentCostSync",
	JobHistory: true,
	Singleton:  true,
	Retention:  job.RetentionBalanced,
	Schedule:   "@every 1h",
	Fn: func(ctx job.JobRuntime) error {
		return ctx.DB().Exec(`
				WITH
				component_children AS (
						SELECT components.id, ARRAY(
								SELECT child_id FROM lookup_component_children(components.id::text, -1)
								UNION
								SELECT relationship_id as child_id FROM component_relationships WHERE component_id IN (
										SELECT child_id FROM lookup_component_children(components.id::text, -1)
								)
						) AS child_ids
						FROM components
						GROUP BY components.id
				),
				component_configs AS (
						SELECT component_children.id, ARRAY_AGG(ccr.config_id) as config_ids
						FROM component_children
						INNER JOIN config_component_relationships ccr ON ccr.component_id = ANY(component_children.child_ids)
						GROUP BY component_children.id
				),
				component_config_costs AS (
						SELECT
								component_configs.id,
								SUM(cost_per_minute) AS cost_per_minute,
								SUM(cost_total_1d) AS cost_total_1d,
								SUM(cost_total_7d) AS cost_total_7d,
								SUM(cost_total_30d) AS cost_total_30d
						FROM config_items
						INNER JOIN component_configs ON config_items.id = ANY(component_configs.config_ids)
						GROUP BY component_configs.id
				)

				UPDATE components
				SET
						cost_per_minute = component_config_costs.cost_per_minute,
						cost_total_1d = component_config_costs.cost_total_1d,
						cost_total_7d = component_config_costs.cost_total_7d,
						cost_total_30d = component_config_costs.cost_total_30d
				FROM component_config_costs
				WHERE components.id = component_config_costs.id
				`).Error
	},
}
