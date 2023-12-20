package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/asecurityteam/rolling"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/duration"
	"github.com/jackc/pgx/v5"
)

type Querier interface {
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
}

func parseDuration(d string, name string) (clause string, arg interface{}, err error) {
	if d == "" {
		return "", nil, nil
	}
	dur, err := duration.ParseDuration(d)
	if err == nil {
		return fmt.Sprintf("(NOW() at TIME ZONE 'utc' - Interval '1 minute' * :%s)", name), dur.Minutes(), nil
	}
	if timestamp, err := time.Parse(time.RFC3339, d); err == nil {
		return ":" + name, timestamp, nil
	}
	return "", nil, fmt.Errorf("start time must be a duration or RFC3339 timestamp")
}

func (q QueryParams) GetWhereClause() (string, map[string]interface{}, error) {
	clause := ""
	args := make(map[string]interface{})
	and := " AND "
	if q.Check != "" {
		clause = "check_id = :check_key"
		args["check_key"] = q.Check
	}
	if q.Start != "" && q.End == "" {
		if clause != "" {
			clause += and
		}
		start, arg, err := parseDuration(q.Start, "start")
		if err != nil {
			return "", nil, err
		}
		args["start"] = arg
		clause += "time > " + start
	} else if q.Start == "" && q.End != "" {
		if clause != "" {
			clause += and
		}
		end, arg, err := parseDuration(q.End, "end")
		if err != nil {
			return "", nil, err
		}
		args["end"] = arg
		clause += "time < " + end
	}
	if q.Start != "" && q.End != "" {
		if clause != "" {
			clause += and
		}
		start, arg, err := parseDuration(q.Start, "start")
		if err != nil {
			return "", nil, err
		}
		args["start"] = arg
		end, arg, err := parseDuration(q.End, "end")
		if err != nil {
			return "", nil, err
		}
		args["end"] = arg
		clause += "time BETWEEN " + start + and + end
	}
	return strings.TrimSpace(clause), args, nil
}

func (q QueryParams) ExecuteDetails(ctx context.Context, db Querier) ([]pkg.Timeseries, pkg.Uptime, pkg.Latency, error) {
	start := q.GetStartTime().Format(time.RFC3339)
	end := q.GetEndTime().Format(time.RFC3339)

	query := `
With grouped_by_window AS (
	SELECT
		duration,
		status,
		CASE  WHEN check_statuses.status = TRUE THEN 1  ELSE 0 END AS passed,
		CASE  WHEN check_statuses.status = FALSE THEN 1  ELSE 0 END AS failed,
		to_timestamp(floor((extract(epoch FROM time) + $1) / $2) * $2) AS time
	FROM check_statuses
	WHERE
		time >= $3 AND
		time <= $4 AND
		check_id = $5
)
SELECT
  time,
  bool_and(status),
  AVG(duration)::integer as duration,
	sum(passed) as passed,
	sum(failed) as failed
FROM
  grouped_by_window
GROUP BY time
ORDER BY time
`
	args := []any{q.WindowDuration.Seconds() / 2, q.WindowDuration.Seconds(), start, end, q.Check}

	if q.WindowDuration == 0 {
		// FIXME
		query = `SELECT time, status, duration,
		CASE  WHEN check_statuses.status = TRUE THEN 1  ELSE 0 END AS passed,
		CASE  WHEN check_statuses.status = FALSE THEN 1  ELSE 0 END AS failed
		FROM check_statuses WHERE time >= $1 AND time <= $2 AND check_id = $3`
		args = []any{start, end, q.Check}
	}
	uptime := pkg.Uptime{}
	latency := rolling.NewPointPolicy(rolling.NewWindow(100))

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, uptime, pkg.Latency{}, err
	}
	defer rows.Close()

	var results []pkg.Timeseries
	for rows.Next() {
		var datapoint pkg.Timeseries
		var ts time.Time
		if err := rows.Scan(&ts, &datapoint.Status, &datapoint.Duration, &datapoint.Passed, &datapoint.Failed); err != nil {
			return nil, uptime, pkg.Latency{}, err
		}
		uptime.Failed += datapoint.Failed
		uptime.Passed += datapoint.Passed
		latency.Append(float64(datapoint.Duration))
		datapoint.Time = ts.Format(time.RFC3339)
		results = append(results, datapoint)
	}

	return results, uptime, pkg.Latency{Percentile95: latency.Reduce(rolling.Percentile(95))}, nil
}
