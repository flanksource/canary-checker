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
		Id:          values.Get("id"),
		TopologyId:  values.Get("topologyId"),
		ComponentId: values.Get("componentId"),
		Status:      values.Get("status"),
	}

	if params.Id != "" && strings.HasPrefix(params.Id, "c-") {
		params.ComponentId = params.Id[2:]
	} else if params.Id != "" {
		params.TopologyId = params.Id
	}
	params.ComponentId = strings.TrimPrefix(params.ComponentId, "c-")

	if depth := values.Get("depth"); depth != "" {
		params.Depth, _ = strconv.Atoi(depth)
	} else {
		params.Depth = DefaultDepth
	}
	return params
}

//nolint
type TopologyParams struct {
	Id          string
	TopologyId  string
	ComponentId string
	Status      string
	Depth       int
}

func (p TopologyParams) String() string {
	s := ""
	if p.TopologyId != "" {
		s += "topologyId=" + p.TopologyId
	}
	if p.ComponentId != "" {
		s += " componentId=" + p.ComponentId
	}
	if p.Depth > 0 {
		s += fmt.Sprintf(" depth=%d", p.Depth)
	}
	if p.Status != "" {
		s += " status=" + p.Status
	}
	return strings.TrimSpace(s)
}

func (p TopologyParams) GetSystemWhereClause() string {
	s := ""
	if p.getID() != "" {
		s = "WHERE templates.id = :id"
	}

	if p.Status != "" {
		if s != "" {
			s += " AND "
		} else {
			s += "WHERE "
		}
		s += "systems.status = :status"
	}
	return s
}

func (p TopologyParams) getID() string {
	if p.Id != "" {
		return p.Id
	}
	if p.ComponentId != "" {
		return p.ComponentId
	}
	if p.TopologyId != "" {
		return p.TopologyId
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
	if p.Status != "" {
		s += " AND components.status = :status or id = :id"
	}
	return s
}

// add a relationship_path on the table...

func (p TopologyParams) GetComponentRelationWhereClause() string {
	s := "where component_relationships.deleted_at is null"
	if p.Status != "" {
		s += " AND parent.status = :status"
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

//	SELECT json_agg(to_jsonb(components)) :: jsonb as components from components LEFT JOIN ((SELECT json_agg(checks) from checks LEFT JOIN check_component_relationships ON checks.id = check_component_relationships.check_id GROUP BY check_component_relationships.component_id) :: jsonb) AS checks ON components.id = checks.component_id

func Query(params TopologyParams) (pkg.Components, error) {
	sql := fmt.Sprintf(`
	SELECT json_agg(jsonb_set_lax(to_jsonb(components),'{checks}', %s)) :: jsonb as components from components %s
	union
	(SELECT json_agg(jsonb_set_lax(jsonb_set_lax(to_jsonb(components), '{parent_id}', to_jsonb(component_relationships.relationship_id), true),'{checks}', %s)) :: jsonb 
	AS components from component_relationships INNER JOIN components 
	ON components.id = component_relationships.component_id INNER JOIN components AS parent 
	ON component_relationships.relationship_id = parent.id %s)
`, getChecksForComponents(), params.GetComponentWhereClause(), getChecksForComponents(), params.GetComponentRelationWhereClause())

	args := make(map[string]interface{})
	if params.Status != "" {
		args["status"] = params.Status
	}
	if params.TopologyId != "" {
		args["id"] = params.TopologyId
	} else if params.ComponentId != "" {
		args["id"] = params.ComponentId
	} else if params.Id != "" {
		args["id"] = params.Id
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
			(SELECT json_agg(checks) from checks LEFT JOIN check_component_relationships ON checks.id = check_component_relationships.check_id WHERE check_component_relationships.component_id = components.id  GROUP BY check_component_relationships.component_id) :: jsonb
			 `
}
