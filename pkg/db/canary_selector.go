package db

import (
	"fmt"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetChecksWithLabelSelector(labelSelector string) (selectedChecks pkg.Checks, err error) {
	if labelSelector == "" {
		return nil, nil
	}
	var uninqueChecks = make(map[string]*pkg.Check)
	matchLabels := GetLabelsFromSelector(labelSelector)
	var labels = make(map[string]string)
	var onlyKeys []string
	for k, v := range matchLabels {
		if v != "" {
			labels[k] = v
		} else {
			onlyKeys = append(onlyKeys, k)
		}
	}
	var checks pkg.Checks
	if err := Gorm.Table("checks").
		Where("labels @> ?", types.JSONStringMap(labels)).
		Where("agent_id = '00000000-0000-0000-0000-000000000000'").
		Where("deleted_at IS NULL").
		Find(&checks).Error; err != nil {
		return nil, err
	}
	for _, c := range checks {
		uninqueChecks[c.ID.String()] = c
	}
	for _, k := range onlyKeys {
		var canaries pkg.Checks
		if err := Gorm.Table("checks").
			Where("labels ?? ?", k).
			Where("agent_id = '00000000-0000-0000-0000-000000000000'").
			Where("deleted_at IS NULL").
			Find(&canaries).Error; err != nil {
			continue
		}
		for _, c := range canaries {
			uninqueChecks[c.ID.String()] = c
		}
	}
	for _, c := range uninqueChecks {
		selectedChecks = append(selectedChecks, c)
	}
	return selectedChecks, nil
}

// returns all the checks associated with canary.
func GetAllChecksForCanary(canaryID uuid.UUID) (checks pkg.Checks, err error) {
	if err := Gorm.Table("checks").Where("canary_id = ?", canaryID).Find(&checks).Error; err != nil {
		return nil, err
	}
	return checks, nil
}

// returns all the checks associated with canary which are currently executing
func GetAllActiveChecksForCanary(canaryID uuid.UUID) (checks pkg.Checks, err error) {
	if err := Gorm.Table("checks").Where("canary_id = ? AND deleted_at is null ", canaryID).Find(&checks).Error; err != nil {
		return nil, err
	}
	return checks, nil
}

func CreateComponentCanaryFromInline(id, name, namespace, schedule, owner string, spec *v1.CanarySpec) (*pkg.Canary, error) {
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
	canary, err := PersistCanary(obj, fmt.Sprintf("component/%s", id))
	if err != nil {
		logger.Debugf("error persisting component inline canary: %v", err)
		return nil, err
	}
	return canary, nil
}
