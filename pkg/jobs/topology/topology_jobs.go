package topology

import (
	"fmt"

	canaryCtx "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty/job"
)

var CleanupDeletedTopologyComponents = &job.Job{
	Name:       "CleanupComponents",
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionBalanced,
	Context:    canaryCtx.DefaultContext,
	Fn: func(ctx job.JobRuntime) error {
		var rows []struct {
			ID string
		}
		// Select all components whose topology ID is deleted but their deleted at is not marked
		if err := ctx.DB().Raw(`
        SELECT DISTINCT(topologies.id) FROM topologies
        INNER JOIN components ON topologies.id = components.topology_id
        WHERE
            components.deleted_at IS NULL AND
            topologies.deleted_at IS NOT NULL
        `).Scan(&rows).Error; err != nil {
			return err
		}

		for _, r := range rows {
			if err := db.DeleteComponentsOfTopology(ctx.DB(), r.ID); err != nil {
				ctx.History.AddError(fmt.Sprintf("Error deleting components for topology[%s]: %v", r.ID, err))
			} else {
				ctx.History.IncrSuccess()
			}
		}
		return nil
	},
}
