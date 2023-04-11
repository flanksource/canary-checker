package api

import (
	"net/http"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/duty/models"
	"github.com/labstack/echo/v4"
)

type Tag struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

type TopologyRes struct {
	componentIDs   []string          `json:"-"`
	Components     models.Components `json:"components,omitempty"`
	HealthStatuses []string          `json:"healthStatuses,omitempty"`
	Teams          []string          `json:"teams,omitempty"`
	Tags           []Tag             `json:"tags,omitempty"`
	Types          []string          `json:"types,omitempty"`
}

func (t *TopologyRes) AddHealthStatuses(s string) {
	for i := range t.HealthStatuses {
		if t.HealthStatuses[i] == s {
			return
		}
	}

	t.HealthStatuses = append(t.HealthStatuses, s)
}

func (t *TopologyRes) AddType(typ string) {
	for i := range t.Types {
		if t.Types[i] == typ {
			return
		}
	}

	t.Types = append(t.Types, typ)
}

func (t *TopologyRes) AddTag(typ map[string]string) {
	for k, v := range typ {
		var exists bool
		for _, tag := range t.Tags {
			if tag.Key == k && tag.Val == v {
				exists = true
				break
			}
		}

		if !exists {
			t.Tags = append(t.Tags, Tag{Key: k, Val: v})
		}
	}
}

// TopologyQuery godoc
// @Id TopologyQuery
// @Summary      Topology query
// @Description Query the topology graph
// @Tags         topology
// @Produce      json
// @Param        id  query   string false "Topology ID"
// @Param        topologyId query  string false "Topology ID"
// @Param        componentId query   string false "Component ID"
// @Param        owner  query  string false "Owner"
// @Param        status  query  string false "Comma separated list of status"
// @Param        types    query string false "Comma separated list of types"
// @Param        flatten  query  string false "Flatten the topology"
// @Success      200  {object}  pkg.Components
// @Router /api/topology [get]
func Topology(c echo.Context) error {
	params := topology.NewTopologyParams(c.QueryParams())
	results, err := topology.Query(params)
	if err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}

	var res TopologyRes
	if len(results) == 0 {
		return c.JSON(http.StatusOK, res)
	}

	res.Components = results
	populateTopologyResult(results, &res)

	res.Teams, err = db.GetTeamsOfComponents(c.Request().Context(), res.componentIDs)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, res)
}

// populateTopologyResult goes through the components recursively (depth-first)
// and populates the TopologyRes struct.
func populateTopologyResult(components models.Components, res *TopologyRes) {
	for _, component := range components {
		res.componentIDs = append(res.componentIDs, component.ID.String())
		res.AddTag(component.Labels)
		res.AddType(component.Type)
		res.AddHealthStatuses(string(component.Status))
		populateTopologyResult(component.Components, res)
	}
}
