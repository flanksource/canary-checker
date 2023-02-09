package topology

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	jsontime "github.com/liamylian/jsontime/v2/v2"
	"github.com/pkg/errors"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
)

var json = jsontime.ConfigWithCustomTimeFormat

const DefaultDepth = 1

func parseItems(items string) []string {
	if strings.TrimSpace(items) == "" {
		return nil
	}
	return strings.Split(items, ",")
}

type TopologyParams struct {
	ID                     string   `query:"id"`
	TopologyID             string   `query:"topologyId"`
	ComponentID            string   `query:"componentId"`
	Owner                  string   `query:"owner"`
	Labels                 string   `query:"labels"`
	Status                 []string `query:"status"`
	Types                  []string `query:"types"`
	Depth                  int      `query:"depth"`
	Flatten                bool     `query:"flatten"`
	IncludeHealth          bool     `query:"includeHealth"`
	IncludeInsightsSummary bool     `query:"includeInsightsSummary"`
}

func NewTopologyParams(values url.Values) TopologyParams {
	params := TopologyParams{
		ID:                     values.Get("id"),
		TopologyID:             values.Get("topologyId"),
		ComponentID:            values.Get("componentId"),
		Status:                 parseItems(values.Get("status")),
		Types:                  parseItems(values.Get("type")),
		Owner:                  values.Get("owner"),
		Flatten:                values.Get("flatten") == "true",
		Labels:                 values.Get("labels"),
		IncludeHealth:          values.Get("includeHealth") != "false",
		IncludeInsightsSummary: values.Get("includeInsightsSummary") != "false",
	}

	if params.ID != "" && strings.HasPrefix(params.ID, "c-") {
		params.ComponentID = params.ID[2:]
	} else if params.ID != "" {
		params.TopologyID = params.ID
	}
	params.ComponentID = strings.TrimPrefix(params.ComponentID, "c-")

	var err error
	if depth := values.Get("depth"); depth != "" {
		params.Depth, err = strconv.Atoi(depth)
		if err != nil {
			params.Depth = DefaultDepth
		}
	} else {
		params.Depth = DefaultDepth
	}
	return params
}

func (p TopologyParams) String() string {
	return fmt.Sprintf("%#v", p)
}

func (p TopologyParams) getLabels() map[string]string {
	if p.Labels == "" {
		return nil
	}
	labels := make(map[string]string)
	for _, label := range strings.Split(p.Labels, ",") {
		parts := strings.Split(label, "=")
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}
	return labels
}

func (p TopologyParams) getID() string {
	if p.ID != "" {
		return p.ID
	}
	if p.ComponentID != "" {
		return p.ComponentID
	}
	if p.TopologyID != "" {
		return p.TopologyID
	}
	return ""
}

func Query(params TopologyParams) (pkg.Components, error) {
	query, args := duty.TopologyQuery(duty.TopologyOptions{
		ID:     params.getID(),
		Owner:  params.Owner,
		Labels: params.getLabels(),
	})
	logger.Infof("Querying topology (%s) => %s", params, query)

	var results pkg.Components
	rows, err := db.QueryNamed(context.Background(), query, args)
	if err != nil {
		return nil, errors.Wrapf(err, "db query failed")
	}
	for rows.Next() {
		var components pkg.Components
		if rows.RawValues()[0] == nil {
			continue
		}
		if err := json.Unmarshal(rows.RawValues()[0], &components); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal components: %s", rows.RawValues()[0])
		}
		results = append(results, components...)
	}

	if !params.Flatten {
		results = results.CreateTreeStructure()
	}

	if params.getID() == "" && len(params.Status) == 0 {
		for _, component := range results.Walk() {
			component.Status = component.GetStatus()
		}
	}

	results = filterComponentsByType(results, params.Types...)
	for _, result := range results {
		result.Components = filterComponentsWithDepth(result.Components, params.Depth)
	}

	results = filterComponentsByStatus(results, params.getID() == "", params.Status...)
	logger.Debugf("Querying topology (%s) => %d components", params, len(results))
	return results, nil
}

func filterComponentsByStatus(components []*pkg.Component, filterRoot bool, statii ...string) []*pkg.Component {
	if len(statii) == 0 {
		return components
	}
	var filtered []*pkg.Component
	for _, component := range components {
		// Filter the root components if requested else filter the 1st level children
		if filterRoot {
			if matchItems(string(component.Status), statii...) {
				filtered = append(filtered, component)
			}
		} else {
			filtered = append(filtered, component)
			var filteredChildren []*pkg.Component
			for _, child := range component.Components {
				if matchItems(string(child.Status), statii...) {
					filteredChildren = append(filteredChildren, child)
				}
			}
			component.Components = filteredChildren
		}
	}
	return filtered
}

func filterComponentsByType(components []*pkg.Component, types ...string) []*pkg.Component {
	if len(types) == 0 {
		return components
	}

	var filtered []*pkg.Component
	for _, component := range components {
		if matchItems(component.Type, types...) {
			filtered = append(filtered, component)
		}
	}
	return filtered
}

// matchItems returns true if any of the items in the list match the item
// negative matches are supported by prefixing the item with a !
// * matches everything
func matchItems(item string, items ...string) bool {
	if len(items) == 0 {
		return true
	}

	for _, i := range items {
		if strings.HasPrefix(i, "!") {
			if item == strings.TrimPrefix(i, "!") {
				return false
			}
		}
	}

	for _, i := range items {
		if strings.HasPrefix(i, "!") {
			continue
		}
		if i == "*" || item == i {
			return true
		}
	}
	return false
}

func filterComponentsWithDepth(components []*pkg.Component, depth int) []*pkg.Component {
	if depth <= 0 || components == nil {
		return components
	}
	if depth == 1 {
		for _, comp := range components {
			comp.Components = nil
		}
	}
	for _, comp := range components {
		comp.Components = filterComponentsWithDepth(comp.Components, depth-1)
	}
	return components
}
