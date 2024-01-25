package canary

import (
	"fmt"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
)

type RelatableCheck interface {
	GetRelationship() *v1.CheckRelationship
}

// formCheckRelationships forms check relationships with components and configs
// based on the lookup expressions in the check spec.
func formCheckRelationships(ctx context.Context, result *pkg.CheckResult) error {
	check := result.Check
	if result.Transformed {
		check = result.ParentCheck // because the parent check has the relationship spec.
	}

	_check, ok := check.(RelatableCheck)
	if !ok {
		return nil
	}
	relationshipConfig := _check.GetRelationship()
	if relationshipConfig == nil {
		return nil
	}

	if result.Canary.GetCheckID(result.Check.GetName()) == "" {
		ctx.Tracef("no canary id found for check %s", result.Check.GetName())
		return nil
	}

	checkID, err := uuid.Parse(result.Canary.GetCheckID(result.Check.GetName()))
	if err != nil {
		return fmt.Errorf("error parsing check id(%s): %w", result.Canary.GetCheckID(result.Check.GetName()), err)
	}

	canaryID, err := uuid.Parse(result.Canary.GetPersistedID())
	if err != nil {
		return fmt.Errorf("error parsing canary id(%s): %w", result.Canary.GetPersistedID(), err)
	}

	for _, lookupSpec := range relationshipConfig.Components {
		componentIDs, err := duty.LookupComponents(ctx, lookupSpec, result.Labels, map[string]any{"result": result})
		if err != nil {
			ctx.Error(err, "error finding components (check=%s) (lookup=%v): %v", checkID, lookupSpec, err)
			continue
		}

		for _, componentID := range componentIDs {
			selectorID, err := utils.GenerateJSONMD5Hash(lookupSpec)
			if err != nil {
				ctx.Error(err, "error generating selector_id hash")
				continue
			}

			rel := &models.CheckComponentRelationship{ComponentID: componentID, CheckID: checkID, CanaryID: canaryID, SelectorID: selectorID}
			if err := rel.Save(ctx.DB()); err != nil {
				ctx.Error(err, "error saving relationship between check=%s and component=%s", checkID, componentID)
			}
		}
	}

	for _, lookupSpec := range relationshipConfig.Configs {
		configIDs, err := duty.LookupConfigs(ctx, lookupSpec, result.Labels, map[string]any{"result": result})
		if err != nil {
			ctx.Error(err, "error finding config items (check=%s) (lookup=%v)", checkID, lookupSpec)
			continue
		}

		for _, configID := range configIDs {
			selectorID, err := utils.GenerateJSONMD5Hash(lookupSpec)
			if err != nil {
				ctx.Error(err, "error generating selector_id hash")
				continue
			}

			rel := &models.CheckConfigRelationship{ConfigID: configID, CheckID: checkID, CanaryID: canaryID, SelectorID: selectorID}
			if err := rel.Save(ctx.DB()); err != nil {
				ctx.Error(err, "error saving relationship between check=%s and config=%s", checkID, configID)
			}
		}
	}

	return nil
}
