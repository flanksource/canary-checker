package topology

import (
	"encoding/json"

	"github.com/flanksource/canary-checker/pkg"
)

func isComponent(s map[string]interface{}) bool {
	_, name := s["name"]
	_, properties := s["properties"]
	return name && properties
}

func isProperty(s map[string]interface{}) bool {
	_, name := s["name"]
	_, properties := s["properties"]
	return name && !properties
}

func isPropertyList(data []byte) bool {
	var s = []map[string]interface{}{}
	if err := json.Unmarshal(data, &s); err != nil {
		return false
	}
	if len(s) == 0 {
		return false
	}
	return isProperty(s[0])
}

func isComponentList(data []byte) bool {
	var s = []map[string]interface{}{}
	if err := json.Unmarshal(data, &s); err != nil {
		return false
	}
	if len(s) == 0 {
		return false
	}
	return isComponent(s[0])
}

func count(components pkg.Components) pkg.Summary {
	s := pkg.Summary{}
	for _, component := range components {
		switch component.Status {
		case "healthy":
			s.Healthy++
		case "unhealthy":
			s.Unhealthy++
		case "warning":
			s.Warning++
		}
		for _, child := range component.Components {
			s = s.Add(count(pkg.Components{child}))
		}
	}

	return s
}
