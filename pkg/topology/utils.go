package topology

import (
	"encoding/json"
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
