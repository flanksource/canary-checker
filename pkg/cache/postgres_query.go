package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/duration"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
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

func (q QueryParams) ExecuteDetails(ctx context.Context, db Querier) ([]pkg.Timeseries, error) {
	start := q.GetStartTime().Format(time.RFC3339)
	end := q.GetEndTime().Format(time.RFC3339)

	query := `
With grouped_by_window AS (
	SELECT
		duration,
		status,
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
  AVG(duration)::integer as duration 
FROM 
  grouped_by_window
GROUP BY time
ORDER BY time
`
	args := []any{q.WindowDuration.Seconds() / 2, q.WindowDuration.Seconds(), start, end, q.Check}

	if q.WindowDuration == 0 {
		query = `SELECT time, status, duration FROM check_statuses WHERE time >= $1 AND time <= $2 AND check_id = $3`
		args = []any{start, end, q.Check}
	}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []pkg.Timeseries
	for rows.Next() {
		var datapoint pkg.Timeseries
		var ts time.Time
		if err := rows.Scan(&ts, &datapoint.Status, &datapoint.Duration); err != nil {
			return nil, err
		}

		datapoint.Time = ts.Format(time.RFC3339)
		results = append(results, datapoint)
	}

	return results, nil
}

func exec(db Querier, q QueryParams, sql string, namedArgs map[string]interface{}) (pgx.Rows, error) {
	if q.Trace {
		sqlDebug := ConvertNamedParamsDebug(sql, namedArgs)
		logger.Tracef(sqlDebug)
	}

	positionalSQL, args := ConvertNamedParams(sql, namedArgs)

	rows, err := db.Query(context.Background(), positionalSQL, args...)

	if err != nil {
		logger.Debugf("Error executing query: %v\n%s\n args=%v", err, positionalSQL, args)
	}
	return rows, err
}

func (q QueryParams) ExecuteSummary(db Querier) (pkg.Checks, error) {
	clause, namedArgs, err := q.GetWhereClause()
	if err != nil {
		return nil, err
	}
	var checkClause string
	if q.CanaryID != "" {
		checkClause += " AND checks.canary_id = :canary_id "
		namedArgs["canary_id"] = q.CanaryID
	}
	if _, exists := namedArgs["check_key"]; exists {
		checkClause += " AND checks.id = :check_key "
	}

	statusColumns := ""
	if q.IncludeMessages {
		statusColumns += ", 'message', message, 'error', error"
	}
	sql := fmt.Sprintf(`
WITH filtered_check_status AS (
    SELECT * FROM check_statuses
    WHERE %s
)
SELECT
    checks.id::text,
    canary_id::text,
    stats.passed,
    stats.failed,
    stats.p99, stats.p97, stats.p95,
    statii,
    type,
    checks.icon,
    checks.name,
    checks.description,
    canaries.namespace,
    canaries.name as canaryName,
    canaries.labels || checks.labels as labels,
    severity,
    owner,
    last_runtime,
    checks.created_at,
    checks.updated_at,
    checks.deleted_at,
    status,
		stats.max_time,
		stats.min_time,
		stats.total_checks
FROM checks checks
RIGHT JOIN (
  	SELECT 
        check_id,
        percentile_disc(0.99) within group (order by filtered_check_status.duration) as p99,
        percentile_disc(0.97) within group (order by filtered_check_status.duration) as p97,
        percentile_disc(0.05) within group (order by filtered_check_status.duration) as p95,
        COUNT(*) FILTER (WHERE filtered_check_status.status = TRUE) as passed,
        COUNT(*) FILTER (WHERE filtered_check_status.status = FALSE) as failed,
				COUNT(*) total_checks,
				MIN(filtered_check_status.time) as min_time,
				MAX(filtered_check_status.time) as max_time
	FROM
        filtered_check_status
    GROUP BY check_id
) as stats ON stats.check_id = checks.id

INNER JOIN canaries on checks.canary_id = canaries.id

RIGHT JOIN (
    SELECT check_id, json_agg(json_build_object('status',status,'duration',duration,'time',time %s)) as statii
	FROM (
		SELECT check_id,
				status,
				time,
				duration,
				message,
				error,
				rank() OVER (
					PARTITION BY check_id
					ORDER BY time DESC
				)
		FROM filtered_check_status
	) check_statuses
	WHERE rank <= :count
	GROUP by check_id
) as statuses ON statuses.check_id = checks.id
WHERE (stats.passed > 0 OR stats.failed > 0) %s
	`, clause, statusColumns, checkClause)

	if q.StatusCount == 0 {
		q.StatusCount = 5
	}
	namedArgs["count"] = q.StatusCount

	rows, err := exec(db, q, sql, namedArgs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	checks := pkg.Checks{}
	for rows.Next() {
		var check = pkg.Check{}
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		check.ID, _ = uuid.Parse(vals[0].(string))
		check.CanaryID, _ = uuid.Parse(vals[1].(string))
		check.Uptime.Passed = intV(vals[2])
		check.Uptime.Failed = intV(vals[3])
		check.Latency.Percentile99 = float64V(vals[4])
		check.Latency.Percentile97 = float64V(vals[5])
		check.Latency.Percentile95 = float64V(vals[6])
		check.Type = vals[8].(string)
		check.Icon = vals[9].(string)
		check.Name = vals[10].(string)
		check.Description = vals[11].(string)
		check.Namespace = vals[12].(string)
		check.CanaryName = vals[13].(string)
		check.Labels = mapStringString(vals[14])
		check.Severity = vals[15].(string)
		check.Owner = vals[16].(string)
		check.LastRuntime, _ = timeV(vals[17])
		check.CreatedAt, _ = timeV(vals[18])
		check.UpdatedAt, _ = timeV(vals[19])
		if vals[20] != nil {
			check.DeletedAt, _ = timeV(vals[20])
		}
		check.Status = vals[21].(string)
		check.LatestRuntime, _ = timeV(vals[22])
		check.EarliestRuntime, _ = timeV(vals[23])
		check.TotalRuns = intV(vals[24])

		if vals[7] != nil {
			for _, status := range vals[7].([]interface{}) {
				s := status.(map[string]interface{})
				check.Statuses = append(check.Statuses, pkg.CheckStatus{
					Status:   s["status"].(bool),
					Time:     s["time"].(string),
					Duration: intV(s["duration"]),
					Message:  stringV(s["message"]),
					Error:    stringV(s["error"]),
				})
			}
		}
		if q.Trace {
			logger.Infof("%+v", check)
		}
		checks = append(checks, &check)
	}
	return checks, err
}
