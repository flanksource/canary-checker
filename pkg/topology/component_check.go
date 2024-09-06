package topology

import (
	"fmt"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComponentCheckRun = &job.Job{
	Name:       "ComponentCheckRun",
	Schedule:   "@every 2m",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionFew,
	Fn: func(run job.JobRuntime) error {
		var components = []pkg.Component{}
		if err := run.DB().Table("components").
			Where("component_checks != 'null'").
			Where(duty.LocalFilter).
			Find(&components).Error; err != nil {
			return fmt.Errorf("error getting components: %v", err)
		}

		for _, component := range components {
			relationships, err := GetChecksForComponent(run.Context, &component)
			if err != nil {
				return err
			}
			err = syncCheckComponentRelationships(run.Context, component, relationships)
			if err != nil {
				run.History.AddError(fmt.Sprintf("error persisting relationships: %v", err))
				continue
			}
			run.History.IncrSuccess()
		}
		return nil
	},
}

func createComponentCanaryFromInline(ctx context.Context, id, name, namespace, schedule, owner string, spec *v1.CanarySpec) (*pkg.Canary, error) {
	if spec.GetSchedule() == "@never" {
		spec.Schedule = schedule
	}
	if spec.Owner == "" {
		spec.Owner = owner
	}
	obj := v1.Canary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *spec,
	}
	canary, _, err := db.PersistCanary(ctx, obj, fmt.Sprintf("component/%s", id))
	if err != nil {
		logger.Errorf("error persisting component inline canary: %v", err)
		return nil, err
	}
	return canary, nil
}

func GetChecksForComponent(ctx context.Context, component *pkg.Component) ([]models.CheckComponentRelationship, error) {
	var relationships []models.CheckComponentRelationship
	for idx, componentCheck := range component.ComponentChecks {
		hash := componentCheck.Hash()
		if componentCheck.Selector.LabelSelector != "" {
			checks, err := query.FindChecks(ctx, -1, componentCheck.Selector)
			if err != nil {
				return nil, err
			}
			for _, check := range checks {
				relationships = append(relationships, models.CheckComponentRelationship{
					CanaryID:    check.CanaryID,
					CheckID:     check.ID,
					ComponentID: component.ID,
					SelectorID:  hash,
				})
			}
		}

		if componentCheck.Inline != nil {
			inlineSchedule := component.Schedule
			if componentCheck.Inline.Schedule != "" {
				inlineSchedule = componentCheck.Inline.Schedule
			}

			canaryName := fmt.Sprintf("%s-%d", component.Name, idx)
			canary, err := createComponentCanaryFromInline(ctx,
				component.ID.String(), canaryName, component.Namespace,
				inlineSchedule, component.Owner, componentCheck.Inline,
			)

			if err != nil {
				return nil, fmt.Errorf("error creating canary from inline: %v", err)
			}

			if err := canaryJobs.SyncCanaryJob(ctx, *canary); err != nil {
				return nil, fmt.Errorf("error creating canary job: %v", err)
			}

			inlineChecks, err := canary.FindChecks(ctx.DB())
			if err != nil {
				return nil, fmt.Errorf("error getting checks for canary: %s. err: %v", canary.ID, err)
			}
			for _, inlineCheck := range inlineChecks {
				relationships = append(relationships, models.CheckComponentRelationship{
					CanaryID:    inlineCheck.CanaryID,
					CheckID:     inlineCheck.ID,
					ComponentID: component.ID,
					SelectorID:  hash,
				})
			}
		}
	}
	return relationships, nil
}

func syncCheckComponentRelationships(ctx context.Context, component pkg.Component, relationships []models.CheckComponentRelationship) error {
	var selectorIDs, checkIDs []string
	existingRelationShips, err := component.GetChecks(ctx.DB())
	if err != nil {
		return err
	}
	db := ctx.DB()
	for _, r := range existingRelationShips {
		selectorIDs = append(selectorIDs, r.SelectorID)
		checkIDs = append(checkIDs, r.CheckID.String())
	}

	var newCheckIDs []string
	for _, r := range relationships {
		newCheckIDs = append(newCheckIDs, r.CheckID.String())

		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, r.SelectorID) {
			continue
		}

		// If checkID does not exist, create a new relationship
		if !collections.Contains(checkIDs, r.CheckID.String()) {
			if err := r.Save(db); err != nil {
				return fmt.Errorf("error persisting check component relationships: %v", err)
			}
			continue
		}

		// If check_id exists mark old row as deleted and update selector_id
		if err := db.Table("check_component_relationships").Where("component_id = ? AND check_id = ?", component.ID, r.CheckID).
			Update("deleted_at", duty.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating check relationships")
		}

		if err := r.Save(db); err != nil {
			return errors.Wrap(err, "error persisting check component relationships")
		}
	}

	// Take set difference of these child component Ids and delete them
	checkIDsToDelete := utils.SetDifference(checkIDs, newCheckIDs)
	if len(checkIDsToDelete) == 0 {
		return nil
	}
	if err := db.Table("check_component_relationships").Where("component_id = ? AND check_id IN ?", component.ID, checkIDsToDelete).
		Update("deleted_at", duty.Now()).Error; err != nil {
		return errors.Wrap(err, "error deleting stale check component relationships")
	}

	return nil
}
