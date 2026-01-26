// ABOUTME: Handles canaries with agentSelector by creating derived canaries for each matched agent.
// ABOUTME: Contains sync and cleanup jobs for managing agent-specific canary instances.

package canary

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/collections"
	dutyjob "github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var SyncAgentSelectorCanaries = &dutyjob.Job{
	Name:       "SyncAgentSelectorCanaries",
	Schedule:   "@every 5m",
	Singleton:  true,
	JobHistory: true,
	RunNow:     true,
	Retention:  dutyjob.RetentionFailed,
	Fn: func(ctx dutyjob.JobRuntime) error {
		canaries, err := db.GetCanariesWithAgentSelector(ctx.Context)
		if err != nil {
			return fmt.Errorf("failed to get canaries with agentSelector: %w", err)
		}

		for _, canary := range canaries {
			if err := syncAgentSelectorCanary(ctx, canary); err != nil {
				ctx.History.AddErrorf("failed to sync canary %s: %v", canary.ID, err)
			} else {
				ctx.History.IncrSuccess()
			}
		}

		return nil
	},
}

func syncAgentSelectorCanary(ctx dutyjob.JobRuntime, parentCanary pkg.Canary) error {
	spec, err := parentCanary.GetSpec()
	if err != nil {
		return fmt.Errorf("failed to get spec: %w", err)
	}

	if len(spec.AgentSelector) == 0 {
		return nil
	}
	source := fmt.Sprintf("agentSelector=%s", parentCanary.ID.String())

	var validAgentIDs []uuid.UUID
	for _, agent := range spec.AgentSelector {
		if val, err := uuid.Parse(agent); err == nil {
			validAgentIDs = append(validAgentIDs, val)
		}
	}

	dbAgents, err := gorm.G[models.Agent](ctx.DB()).Select("id", "name").Where("deleted_at IS NULL").Find(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agents: %w", err)
	}
	for _, dbAgent := range dbAgents {
		if matches := collections.MatchItems(dbAgent.Name, spec.AgentSelector...); matches {
			validAgentIDs = append(validAgentIDs, dbAgent.ID)
		}
	}

	spec.AgentSelector = nil
	newSpec, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("error marshaling spec for canary[%s]: %w", parentCanary.ID, err)
	}
	parentCanary.Spec = newSpec

	for _, agentID := range validAgentIDs {
		if err := upsertDerivedCanary(ctx, parentCanary, agentID, source); err != nil {
			return fmt.Errorf("failed to upsert canary for agent[%s]: %w", agentID, err)
		}
	}

	if err := deleteDerivedCanariesNotInList(ctx, source, validAgentIDs); err != nil {
		return fmt.Errorf("failed to cleanup old derived canaries: %w", err)
	}

	return nil
}

func upsertDerivedCanary(ctx dutyjob.JobRuntime, parentCanary pkg.Canary, agentID uuid.UUID, source string) error {
	var existing pkg.Canary
	err := ctx.DB().
		Where("source = ? AND agent_id = ? AND deleted_at IS NULL", source, agentID).
		First(&existing).Error

	if err == nil {
		return ctx.DB().Model(&existing).Updates(map[string]any{
			"spec":        parentCanary.Spec,
			"labels":      parentCanary.Labels,
			"annotations": parentCanary.Annotations,
		}).Error
	}

	derivedCanary := pkg.Canary{
		AgentID:     agentID,
		Name:        parentCanary.Name,
		Namespace:   parentCanary.Namespace,
		Spec:        parentCanary.Spec,
		Labels:      parentCanary.Labels,
		Annotations: parentCanary.Annotations,
		Source:      source,
	}

	return ctx.DB().Create(&derivedCanary).Error
}

func deleteDerivedCanariesNotInList(ctx dutyjob.JobRuntime, source string, keepAgentIDs []uuid.UUID) error {
	q := ctx.DB().Table("canaries").
		Where("source = ?", source).
		Where("deleted_at IS NULL")

	if len(keepAgentIDs) > 0 {
		q = q.Where("agent_id NOT IN ?", keepAgentIDs)
	}

	return q.Update("deleted_at", time.Now()).Error
}

var CleanupOrphanedAgentSelectorCanaries = &dutyjob.Job{
	Name:       "CleanupOrphanedAgentSelectorCanaries",
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Retention:  dutyjob.RetentionFailed,
	Fn: func(ctx dutyjob.JobRuntime) error {
		return ctx.DB().Exec(`
			UPDATE canaries
			SET deleted_at = NOW()
			WHERE deleted_at IS NULL
			  AND source LIKE 'agentSelector=%'
			  AND NOT EXISTS (
				SELECT 1
				FROM canaries AS parent
				WHERE parent.id = CAST(REPLACE(canaries.source, 'agentSelector=', '') AS UUID)
				  AND parent.deleted_at IS NULL
			  )`).Error
	},
}
