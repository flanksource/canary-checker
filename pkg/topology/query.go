package topology

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/flanksource/commons/collections"
	dutyContext "github.com/flanksource/duty/context"
	dutyQuery "github.com/flanksource/duty/query"
)

const DefaultDepth = 3

func NewTopologyParams(values url.Values) dutyQuery.TopologyOptions {
	parseItems := func(items string) []string {
		if strings.TrimSpace(items) == "" {
			return nil
		}
		return strings.Split(strings.TrimSpace(items), ",")
	}

	var labels map[string]string
	if values.Get("labels") != "" {
		labels = collections.KeyValueSliceToMap(strings.Split(values.Get("labels"), ","))
	}

	var err error
	var depth = DefaultDepth
	if depthStr := values.Get("depth"); depthStr != "" {
		depth, err = strconv.Atoi(depthStr)
		if err != nil {
			depth = DefaultDepth
		}
	}
	return dutyQuery.TopologyOptions{
		ID:        values.Get("id"),
		Owner:     values.Get("owner"),
		Labels:    labels,
		Status:    parseItems(values.Get("status")),
		Depth:     depth,
		Types:     parseItems(values.Get("type")),
		Flatten:   values.Get("flatten") == "true",
		SortBy:    dutyQuery.TopologyQuerySortBy(values.Get("sortBy")),
		SortOrder: values.Get("sortOrder"),
	}
}

func Query(ctx dutyContext.Context, params dutyQuery.TopologyOptions) (*dutyQuery.TopologyResponse, error) {
	return dutyQuery.Topology(ctx, params)
}
