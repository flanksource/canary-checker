package api

import (
	"fmt"
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

type TopologyResponse struct {
	componentIDs    []string
	healthStatusMap map[string]struct{}
	typeMap         map[string]struct{}
	tagMap          map[string]struct{}

	Components     models.Components `json:"components,omitempty"`
	HealthStatuses []string          `json:"healthStatuses,omitempty"`
	Teams          []string          `json:"teams,omitempty"`
	Tags           []Tag             `json:"tags,omitempty"`
	Types          []string          `json:"types,omitempty"`
}

func (t *TopologyResponse) AddHealthStatuses(s string) {
	if t.healthStatusMap == nil {
		t.healthStatusMap = make(map[string]struct{})
	}

	if _, exists := t.healthStatusMap[s]; !exists {
		t.HealthStatuses = append(t.HealthStatuses, s)
		t.healthStatusMap[s] = struct{}{}
	}
}

func (t *TopologyResponse) AddType(typ string) {
	if t.typeMap == nil {
		t.typeMap = make(map[string]struct{})
	}

	if _, exists := t.typeMap[typ]; !exists {
		t.Types = append(t.Types, typ)
		t.typeMap[typ] = struct{}{}
	}
}

func (t *TopologyResponse) AddTag(tags map[string]string) {
	if t.tagMap == nil {
		t.tagMap = make(map[string]struct{})
	}

	for k, v := range tags {
		tagKey := fmt.Sprintf("%s=%s", k, v)
		if _, exists := t.tagMap[tagKey]; !exists {
			t.Tags = append(t.Tags, Tag{Key: k, Val: v})
			t.tagMap[tagKey] = struct{}{}
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

	var res TopologyResponse
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
func populateTopologyResult(components models.Components, res *TopologyResponse) {
	for _, component := range components {
		res.componentIDs = append(res.componentIDs, component.ID.String())
		res.AddTag(component.Labels)
		res.AddType(component.Type)
		res.AddHealthStatuses(string(component.Status))
		populateTopologyResult(component.Components, res)
	}
}
