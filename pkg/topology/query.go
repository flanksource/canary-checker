package topology

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/models"
)

const DefaultDepth = 1

type TopologyParams struct {
	ID                     string   `query:"id"`
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
	parseItems := func(items string) []string {
		return strings.Split(strings.TrimSpace(items), ",")
	}

	params := TopologyParams{
		ID:                     values.Get("id"),
		Status:                 parseItems(values.Get("status")),
		Types:                  parseItems(values.Get("type")),
		Owner:                  values.Get("owner"),
		Flatten:                values.Get("flatten") == "true",
		Labels:                 values.Get("labels"),
		IncludeHealth:          values.Get("includeHealth") != "false",
		IncludeInsightsSummary: values.Get("includeInsightsSummary") != "false",
	}

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

func Query(params TopologyParams) (models.Components, error) {
	return duty.QueryTopology(db.Pool, duty.TopologyOptions{
		ID:      params.ID,
		Owner:   params.Owner,
		Labels:  collections.KeyValueSliceToMap(strings.Split(params.Labels, ",")),
		Status:  params.Status,
		Depth:   params.Depth,
		Types:   params.Types,
		Flatten: params.Flatten,
	})
}
