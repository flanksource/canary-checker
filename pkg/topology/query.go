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
	s := "where deleted_at is null "
	if p.getID() != "" {
		s += " and (starts_with(path, (select concat(path,'.', id) as path from components where id = :id)) or id = :id or path = :id :: text)"
	}
	if p.Status != "" {
		s += " AND components.status = :status"
	}
	return s
}

func Query(params TopologyParams) (pkg.Components, error) {
	sql := fmt.Sprintf(`
	Select json_agg(To_json(components)) :: jsonb as components from components 
%s`, params.GetComponentWhereClause())

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
		if err := json.Unmarshal(rows.RawValues()[0], &components); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal components: %s", rows.RawValues()[0])
		}
		results = append(results, components...)
	}
	return results, nil
}
