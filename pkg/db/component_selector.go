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

func GetComponentsWithLabelSelector(labelSelector string) (components pkg.Components, err error) {
	if labelSelector == "" {
		return nil, nil
	}
	var uninqueComponents = make(map[string]*pkg.Component)
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
	var comps pkg.Components
	if err := Gorm.Table("components").Where("labels @> ? and deleted_at is null", types.JSONStringMap(labels)).Find(&comps).Error; err != nil {
		return nil, err
	}
	for _, c := range comps {
		uninqueComponents[c.ID.String()] = c
	}
	for _, k := range onlyKeys {
		var comps pkg.Components
		if err := Gorm.Table("components").Where("labels ?? ? and deleted_at is null", k).Find(&comps).Error; err != nil {
			continue
		}
		for _, c := range comps {
			uninqueComponents[c.ID.String()] = c
		}
	}
	for _, c := range uninqueComponents {
		components = append(components, c)
	}
	return components, nil
}

func GetComponentsWithFieldSelector(fieldSelector string) (components pkg.Components, err error) {
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
