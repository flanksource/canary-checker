package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/pkg/errors"
)

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
		s = "WHERE system.id = :id"
	}

	if p.Status != "" {
		if s != "" {
			s += " AND "
		} else {
			s += "WHERE "
		}
		s += "system.status = :status"
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
	if p.getID() == "" {
		return ""
	}
	s := `
UNION
SELECT NULL :: jsonb                         AS system,
	json_agg(To_json(component)) :: jsonb AS components
FROM component
WHERE component.id = :id OR component.parent_id = :id`

	if p.Status != "" {
		s += " AND component.status = :status"
	}
	return s
}

func Query(params TopologyParams) ([]pkg.System, error) {
	sql := fmt.Sprintf(`
SELECT
	to_json(systems) :: jsonb       AS system,
	components.components :: jsonb AS components
FROM   systems
	full join (SELECT system_id,
										json_agg(To_json(components)) AS components
						 FROM   components
						 GROUP  BY system_id) AS components
	ON systems.id = components.system_id
%s
%s`, params.GetSystemWhereClause(), params.GetComponentWhereClause())

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

	var results []pkg.System
	rows, err := db.QueryNamed(context.Background(), sql, args)
	if err != nil {
		return nil, errors.Wrapf(err, "db query failed")
	}
	for rows.Next() {
		system := &pkg.System{}
		if len(rows.RawValues()[0]) > 0 {
			if err := json.Unmarshal(rows.RawValues()[0], system); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal: %s", rows.RawValues()[0])
			}
		}
		if len(rows.RawValues()[1]) != 0 {
			var components pkg.Components
			if err := json.Unmarshal(rows.RawValues()[1], &components); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal components: %s", rows.RawValues()[1])
			}
			system.Components = components
			for i := range system.Components {
				if system.Components[i].ParentId.String() == "" {
					if system.ID.String() != "" {
						system.Components[i].ParentId = &system.ID
					}
				}
			}
		}

		results = append(results, *system)
	}

	return results, nil
}
