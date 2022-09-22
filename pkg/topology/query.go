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
)

var json = jsontime.ConfigWithCustomTimeFormat

const DefaultDepth = 1

func parseItems(items string) []string {
	if strings.TrimSpace(items) == "" {
		return nil
	}
	return strings.Split(items, ",")
}

func NewTopologyParams(values url.Values) TopologyParams {
	params := TopologyParams{
		ID:          values.Get("id"),
		TopologyID:  values.Get("topologyId"),
		ComponentID: values.Get("componentId"),
		Status:      parseItems(values.Get("status")),
		Types:       parseItems(values.Get("type")),
		Owner:       values.Get("owner"),
		Flatten:     values.Get("flatten") == "true",
		Labels:      values.Get("labels"),
	}

	if params.ID != "" && strings.HasPrefix(params.ID, "c-") {
		params.ComponentID = params.ID[2:]
	} else if params.ID != "" {
		params.TopologyID = params.ID
	}
	params.ComponentID = strings.TrimPrefix(params.ComponentID, "c-")

	if depth := values.Get("depth"); depth != "" {
		params.Depth, _ = strconv.Atoi(depth)
	} else {
		params.Depth = DefaultDepth
	}
	return params
}

type TopologyParams struct {
	ID          string   `query:"id"`
	TopologyID  string   `query:"topologyId"`
	ComponentID string   `query:"componentId"`
	Owner       string   `query:"owner"`
	Labels      string   `query:"labels"`
	Status      []string `query:"status"`
	Types       []string `query:"types"`
	Depth       int      `query:"depth"`
	Flatten     bool     `query:"flatten"`
}

func (p TopologyParams) String() string {
	s := ""
	if p.ID != "" {
		s += fmt.Sprintf("id=%s ", p.ID)
	}
	if p.TopologyID != "" {
		s += "topologyId=" + p.TopologyID
	}
	if p.ComponentID != "" {
		s += " componentId=" + p.ComponentID
	}
	if p.Depth > 0 {
		s += fmt.Sprintf(" depth=%d", p.Depth)
	}
	if len(p.Status) > 0 {
		s += " status=" + strings.Join(p.Status, ",")
	}
	if len(p.Types) > 0 {
		s += " types=" + strings.Join(p.Types, ",")
	}
	if p.Flatten {
		s += " flatten=true"
	}
	if p.Labels != "" {
		s += " labels=" + p.Labels
	}
	if p.Owner != "" {
		s += " owner=" + p.Owner
	}
	return strings.TrimSpace(s)
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

func (p TopologyParams) GetComponentWhereClause() string {
	s := "where components.deleted_at is null "
	if p.getID() != "" {
		s += `and (starts_with(path,
			(SELECT
				(CASE WHEN (path IS NULL OR path = '') THEN id :: text ELSE concat(path,'.', id) END)
				FROM components where id = :id)
			) or id = :id or path = :id :: text)`
	}
	if p.Owner != "" {
		s += " AND (components.owner = :owner or id = :id)"
	}
	if p.Labels != "" {
		s += " AND (components.labels @> :labels or id = :id)"
	}
	return s
}

func (p TopologyParams) GetComponentRelationWhereClause() string {
	s := "where component_relationships.deleted_at is null"
	if p.Owner != "" {
		s += " AND (parent.owner = :owner)"
	}
	if p.Labels != "" {
		s += " AND (parent.labels @> :labels)"
	}
	if p.getID() != "" {
		s += ` and (component_relationships.relationship_id = :id or starts_with(component_relationships.relationship_path, (SELECT
			(CASE WHEN (path IS NULL OR path = '') THEN id :: text ELSE concat(path,'.', id) END)
			FROM components where id = :id)))`
	} else {
		s += ` and (parent.parent_id is null or starts_with(component_relationships.relationship_path, (SELECT
			(CASE WHEN (path IS NULL OR path = '') THEN id :: text ELSE concat(path,'.', id) END)
			FROM components where id = parent.id)))`
	}
	return s
}

func Query(params TopologyParams) (pkg.Components, error) {
	query := fmt.Sprintf(`
	SELECT json_agg(
        jsonb_set_lax(
            jsonb_set_lax(
                to_jsonb(components),'{checks}', %s
            ), '{configs}', %s
        )
    ) :: jsonb AS components FROM components %s
	UNION (
    SELECT json_agg(
        jsonb_set_lax(
            jsonb_set_lax(
                jsonb_set_lax(
                    to_jsonb(components), '{parent_id}', to_jsonb(component_relationships.relationship_id), true
                ),'{checks}', %s
            ), '{configs}', %s
        )
    ):: jsonb AS components FROM component_relationships INNER JOIN components
	ON components.id = component_relationships.component_id INNER JOIN components AS parent
	ON component_relationships.relationship_id = parent.id %s)`,
		getChecksForComponents(), getConfigForComponents(), params.GetComponentWhereClause(),
		getChecksForComponents(), getConfigForComponents(), params.GetComponentRelationWhereClause())

	args := make(map[string]interface{})
	if params.getID() != "" {
		args["id"] = params.getID()
	}
	if params.Owner != "" {
		args["owner"] = params.Owner
	}
	if params.Labels != "" {
		args["labels"] = params.getLabels()
	}

	logger.Tracef("Querying topology (%s) => %s", params, sql)

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

	results = filterComponentsByStatus(results, params.Status...)

	logger.Debugf("Querying topology (%s) => %d components", params, len(results))
	return results, nil
}

func filterComponentsByStatus(components []*pkg.Component, statii ...string) []*pkg.Component {
	if len(statii) == 0 {
		return components
	}
	var filtered []*pkg.Component
	for _, component := range components {
		if matchItems(string(component.Status), statii...) {
			filtered = append(filtered, component)
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

func getChecksForComponents() string {
	return `
			(SELECT json_agg(checks) from checks LEFT JOIN check_component_relationships ON checks.id = check_component_relationships.check_id WHERE check_component_relationships.component_id = components.id AND check_component_relationships.deleted_at is null   GROUP BY check_component_relationships.component_id) :: jsonb
			 `
}

func getConfigForComponents() string {
	return `
       (SELECT json_agg(config_items.config) from config_items
        LEFT JOIN config_component_relationships ON config_items.id = config_component_relationships.config_id
        WHERE config_component_relationships.component_id = components.id AND config_component_relationships.deleted_at IS NULL
        GROUP BY config_component_relationships.component_id) :: jsonb
	`
}
