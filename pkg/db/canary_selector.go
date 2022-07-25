package db

import (
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/google/uuid"
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
