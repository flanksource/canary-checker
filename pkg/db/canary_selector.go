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

func GetCanariesWithLabelSelector(labelSelector string) (selectedCanaries []pkg.Canary, err error) {
	if labelSelector == "" {
		return nil, nil
	}
	var uninqueCanaries = make(map[string]pkg.Canary)
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
	var canaries []pkg.Canary
	if err := Gorm.Table("canaries").Where("labels @> ?", types.JSONStringMap(labels)).Find(&canaries).Error; err != nil {
		return nil, err
	}
	for _, c := range canaries {
		uninqueCanaries[c.ID.String()] = c
	}
	for _, k := range onlyKeys {
		var canaries []pkg.Canary
		if err := Gorm.Table("canaries").Where("labels ?? ?", k).Find(&canaries).Error; err != nil {
			continue
		}
		for _, c := range canaries {
			uninqueCanaries[c.ID.String()] = c
		}
	}
	for _, c := range uninqueCanaries {
		selectedCanaries = append(selectedCanaries, c)
	}
	return selectedCanaries, nil
}

func GetAllChecksForCanary(canaryID uuid.UUID) (checks []pkg.Check, err error) {
	if err := Gorm.Table("checks").Where("canary_id = ?", canaryID).Find(&checks).Error; err != nil {
		return nil, err
	}
	return checks, nil
}

func CreateComponentCanaryFromInline(name, namespace, schedule string, spec *v1.CanarySpec) ([]pkg.Canary, error) {
	if spec.GetSchedule() == "@never" {
		fmt.Println("setting the schedule here to", schedule)
		spec.Schedule = schedule
	}
	obj := v1.Canary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *spec,
	}
	id, _, err := PersistCanary(obj, "component/inline")
	if err != nil {
		logger.Debugf("error persisting component inline canary: %v", err)
		return nil, err
	}
	canary, err := GetCanary(id)
	if err != nil {
		logger.Debugf("error getting component inline canary: %v", err)
		return nil, err
	}
	return []pkg.Canary{*canary}, nil
}
