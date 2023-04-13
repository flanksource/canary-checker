package topology

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/duty"
)

const DefaultDepth = 3

func NewTopologyParams(values url.Values) duty.TopologyOptions {
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
	return duty.TopologyOptions{
		ID:      values.Get("id"),
		Owner:   values.Get("owner"),
		Labels:  labels,
		Status:  parseItems(values.Get("status")),
		Depth:   depth,
		Types:   parseItems(values.Get("type")),
		Flatten: values.Get("flatten") == "true",
	}
}

func Query(params duty.TopologyOptions) (*duty.TopologyResponse, error) {
	return duty.QueryTopology(db.Pool, params)
}
