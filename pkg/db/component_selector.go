package db

import (
	"strings"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/types"
)

func GetLabelsFromSelector(selector string) (matchLabels map[string]string) {
	matchLabels = make(types.JSONStringMap)
	labels := strings.Split(selector, ",")
	for _, label := range labels {
		if strings.Contains(label, "=") {
			kv := strings.Split(label, "=")
			if len(kv) == 2 {
				matchLabels[kv[0]] = kv[1]
			} else {
				matchLabels[kv[0]] = ""
			}
		}
	}
	return
}

func GetComponensWithLabelSelector(labelSelector string) (components pkg.Components, err error) {
	if labelSelector == "" {
		return nil, nil
	}
	var uninqueComponents = make(map[string]*pkg.Component)
	matchLabels := GetLabelsFromSelector(labelSelector)
	for k, v := range matchLabels {
		var comp pkg.Components
		if v != "" {
			if err := Gorm.Table("components").Where("labels @> ?", types.JSONStringMap{k: v}).Find(&comp).Error; err != nil {
				continue
			}
			for _, c := range comp {
				uninqueComponents[c.ID.String()] = c
			}
		} else {
			if err := Gorm.Table("components").Where("labels ?? ?", k).Find(&comp).Error; err != nil {
				continue
			}
			for _, c := range comp {
				uninqueComponents[c.ID.String()] = c
			}
		}
	}
	for _, c := range uninqueComponents {
		components = append(components, c)
	}
	return
}

func GetComponensWithFieldSelector(fieldSelector string) (components pkg.Components, err error) {
	if fieldSelector == "" {
		return nil, nil
	}
	var uninqueComponents = make(map[string]*pkg.Component)
	matchLabels := GetLabelsFromSelector(fieldSelector)
	for k, v := range matchLabels {
		var comp pkg.Components
		Gorm.Raw("select * from lookup_component_by_property(?, ?)", k, v).Scan(&comp)
		for _, c := range comp {
			uninqueComponents[c.ID.String()] = c
		}
	}
	for _, c := range uninqueComponents {
		components = append(components, c)
	}
	return
}
