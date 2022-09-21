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

func NewTopologyParams(values url.Values) TopologyParams {
	params := TopologyParams{
		ID:          values.Get("id"),
		TopologyID:  values.Get("topologyId"),
		ComponentID: values.Get("componentId"),
		Status:      values.Get("status"),
		Owner:       values.Get("owner"),
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
	ID          string
	TopologyID  string
	ComponentID string
	Owner       string
	Labels      string
	Status      string
	Depth       int
}

func (p TopologyParams) String() string {
	s := ""
	if p.TopologyID != "" {
		s += "topologyId=" + p.TopologyID
	}
	if p.ComponentID != "" {
		s += " componentId=" + p.ComponentID
	}
	if p.Depth > 0 {
		s += fmt.Sprintf(" depth=%d", p.Depth)
	}
	if p.Status != "" {
		s += " status=" + p.Status
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
	sql := fmt.Sprintf(`
	SELECT json_agg(jsonb_set_lax(to_jsonb(components),'{checks}', %s)) :: jsonb as components from components %s
	union
	(SELECT json_agg(jsonb_set_lax(jsonb_set_lax(to_jsonb(components), '{parent_id}', to_jsonb(component_relationships.relationship_id), true),'{checks}', %s)) :: jsonb 
	AS components from component_relationships INNER JOIN components 
	ON components.id = component_relationships.component_id INNER JOIN components AS parent 
	ON component_relationships.relationship_id = parent.id %s)
    UNION
	SELECT json_agg(jsonb_set_lax(to_jsonb(components),'{configs}', %s)) :: jsonb as components from components %s
`, getChecksForComponents(), params.GetComponentWhereClause(), getChecksForComponents(),
		params.GetComponentRelationWhereClause(), getConfigForComponents(), params.GetComponentWhereClause())

	args := make(map[string]interface{})
	if params.getID() != "" {
		args["id"] = params.getID()
	}
	if params.Owner != "" {
		args["owner"] = params.Owner
	}
	if params.Labels != "" {
		fmt.Println(params.Labels)
		args["labels"] = params.getLabels()
	}

	logger.Infof("Querying topology (%s) => %s", params, sql)

	var results pkg.Components
	rows, err := db.QueryNamed(context.Background(), sql, args)
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

	results = results.CreateTreeStructure(params.getID(), params.Status)
	for _, result := range results {
		result.Components = filterComponentsWithDepth(result.Components, params.Depth)
	}
	if params.Status != "" {
		results = results.FilterChildByStatus(pkg.ComponentStatus(params.Status))
	}
	return results, nil
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
