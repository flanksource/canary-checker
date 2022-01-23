package topology

import (
	"context"
	"encoding/json"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
)

type TopologyParams struct {
}

func Query(params TopologyParams) ([]pkg.System, error) {
	sql := `select jsonb_set(to_json(system)::jsonb, '{components}', components.components::jsonb, true) as system FROM system
		FULL JOIN (select system_id, json_agg(to_json(component)) as components from component group by system_id) as components
			on system.id = components.system_id`

	var results []pkg.System
	rows, err := db.Pool.Query(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		system := pkg.System{}

		if err := json.Unmarshal(rows.RawValues()[0], &system); err != nil {
			return nil, err
		}
		results = append(results, system)
	}

	return results, nil
}
