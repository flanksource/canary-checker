package checks

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	gocache "github.com/patrickmn/go-cache"
)

var checksCache = gocache.New(5*time.Minute, 5*time.Minute)

var DisabledChecks []string

func getDisabledChecks(ctx *context.Context) (map[string]struct{}, error) {
	if val, ok := checksCache.Get("disabledChecks"); ok {
		return val.(map[string]struct{}), nil
	}

	result := make(map[string]struct{})
	if ctx.DB() == nil {
		return result, nil
	}

	rows, err := ctx.DB().Raw("SELECT name FROM properties WHERE name LIKE 'check.disabled.%' AND value = 'true'").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		result[strings.TrimPrefix(name, "check.disabled.")] = struct{}{}
	}

	for _, check := range DisabledChecks {
		result[check] = struct{}{}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	checksCache.SetDefault("disabledChecks", result)
	return result, nil
}

// Exec runs the actions specified and returns the results, without exporting any metrics
func Exec(ctx *context.Context) ([]*pkg.CheckResult, error) {
	var results []*pkg.CheckResult
	disabledChecks, err := getDisabledChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting disabled checks: %v", err)
	}

	checks := ctx.Canary.Spec.GetAllChecks()
	for _, c := range All {
		// FIXME: this doesn't work correct with DNS,
		// t := GetDeadline(ctx.Canary)
		// ctx, cancel := ctx.WithDeadline(t)
		// defer cancel()

		if _, ok := disabledChecks[c.Type()]; ok {
			continue
		}
		if !Checks(checks).Includes(c) {
			continue
		}

		result := c.Run(ctx)
		transformedResults := TransformResults(ctx, result)
		results = append(results, transformedResults...)
	}
	return results, nil
}

func RunChecks(ctx *context.Context) ([]*pkg.CheckResult, error) {
	var results []*pkg.CheckResult
	disabledChecks, err := getDisabledChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting disabled checks: %v", err)
	}

	// Check if canary is not marked deleted in DB
	if ctx.DB() != nil && ctx.Canary.GetPersistedID() != "" {
		var deletedAt sql.NullTime
		err := ctx.DB().Table("canaries").Select("deleted_at").Where("id = ? and deleted_at < now()", ctx.Canary.GetPersistedID()).Scan(&deletedAt).Error
		if err != nil {
			return nil, fmt.Errorf("error getting canary: %v", err)
		}

		if deletedAt.Valid {
			return nil, nil
		}
	}

	checks := ctx.Canary.Spec.GetAllChecks()
	ctx.Debugf("[%s] checking %d checks", ctx.Canary.Name, len(checks))
	for _, c := range All {
		// FIXME: this doesn't work correct with DNS,
		// t := GetDeadline(ctx.Canary)
		// ctx, cancel := ctx.WithDeadline(t)
		// defer cancel()

		if _, ok := disabledChecks[c.Type()]; ok {
			continue
		}
		if !Checks(checks).Includes(c) {
			continue
		}

		result := c.Run(ctx)
		transformedResults := TransformResults(ctx, result)
		results = append(results, transformedResults...)
		ExportCheckMetrics(ctx, transformedResults)
	}

	if err := saveArtifacts(ctx, results); err != nil {
		ctx.Errorf("error saving artifacts: %v", err)
	}

	return ProcessResults(ctx, results), nil
}

func saveArtifacts(ctx *context.Context, results pkg.Results) error {
	if results.TotalArtifacts() == 0 {
		return nil
	}

	if DefaultArtifactConnection == "" {
		ctx.Warnf("no artifact connection configured")
		return nil
	}

	connection, err := ctx.HydrateConnectionByURL(DefaultArtifactConnection)
	if err != nil {
		return fmt.Errorf("error getting connection(%s): %w", DefaultArtifactConnection, err)
	} else if connection == nil {
		return fmt.Errorf("connection(%s) was not found", DefaultArtifactConnection)
	}

	fs, err := artifacts.GetFSForConnection(ctx.Context, *connection)
	if err != nil {
		return fmt.Errorf("error getting filesystem for connection: %w", err)
	}
	defer fs.Close()

	for _, r := range results {
		if len(r.Artifacts) == 0 {
			continue
		}

		checkIDRaw := r.Canary.GetCheckID(r.Check.GetName())
		checkID, err := uuid.Parse(checkIDRaw)
		if err != nil {
			ctx.Error(err, "error parsing checkID %s", checkIDRaw)
			continue
		}

		for _, a := range r.Artifacts {
			a.Path = filepath.Join("checks", checkID.String(), fmt.Sprintf("%d", r.Start.UnixNano()), a.Path)
			artifact := models.Artifact{
				CheckID:      utils.Ptr(checkID),
				CheckTime:    utils.Ptr(r.Start),
				ConnectionID: connection.ID,
			}
			if err := artifacts.SaveArtifact(ctx.Context, fs, &artifact, a); err != nil {
				return fmt.Errorf("error saving artifact to db: %w", err)
			}
		}
	}

	return nil
}

func TransformResults(ctx *context.Context, in []*pkg.CheckResult) (out []*pkg.CheckResult) {
	for _, r := range in {
		if r.Invalid {
			out = append(out, r)
			continue
		}
		checkCtx := ctx.WithCheckResult(r)
		transformed, hasTransformer, err := transform(checkCtx, r)
		if hasTransformer {
			// Keeping the detail field empty as it can have huge json blobs
			r.Detail = nil

			// We'll append the original check status to the result
			// so that it can be tracked
			out = append(out, r)
		}
		if err != nil {
			r.ErrorObject = ctx.Oops().Wrap(err)
			r.Error = fmt.Sprintf("error transforming result: %s", err.Error())
			r.Invalid = true
			r.Pass = false
		} else {
			for _, t := range transformed {
				out = append(out, processTemplates(checkCtx, t))
			}
		}
	}
	return out
}

func ProcessResults(ctx *context.Context, results []*pkg.CheckResult) []*pkg.CheckResult {
	if ctx.Canary.Spec.ResultMode == "" {
		return results
	}
	switch ctx.Canary.Spec.ResultMode {
	case v1.JunitResultMode:
		suite := GetJunitReportFromResults(ctx.Canary.GetName(), results)
		var status = true
		if suite.Failed > 0 {
			status = false
		}
		return []*pkg.CheckResult{
			{
				Pass:   status,
				Canary: ctx.Canary,
				Detail: suite,
				Check: v1.JunitCheck{
					TestResults: "combined",
					Description: v1.Description{Description: "Result Mode: JUnit Report"},
				},
				Message: suite.String(),
				Start:   time.Now(),
			},
		}
	default:
		ctx.Errorf("Unknown result mode: %s", ctx.Canary.Spec.ResultMode)
	}

	return results
}

func contextMapToSlice(c map[string]any) []any {
	args := []any{}
	for k, v := range c {
		args = append(args, k, v)
	}
	return args
}

func processTemplates(ctx *context.Context, r *pkg.CheckResult) *pkg.CheckResult {
	if r.Invalid {
		return r
	}
	if r.Duration == 0 && r.GetDuration() > 0 {
		r.Duration = r.GetDuration()
	}
	switch v := r.Check.(type) {
	case v1.DisplayTemplate:
		if !v.GetDisplayTemplate().IsEmpty() {
			message, err := template(ctx, v.GetDisplayTemplate())
			if err != nil {
				r.ErrorMessage(ctx.Oops().With(contextMapToSlice(r.GetContext())...).Wrap(err))
			} else {
				r.ResultMessage(message)
			}
		}
	}

	switch v := r.Check.(type) {
	case v1.TestFunction:
		tpl := v.GetTestFunction()
		if tpl.IsEmpty() {
			break
		}

		message, err := template(ctx, tpl)
		if err != nil {
			r.ErrorMessage(ctx.Oops().With(contextMapToSlice(r.GetContext())...).Wrap(err))
		} else if parsed, err := strconv.ParseBool(message); err != nil {
			r.Failf("test expression did not return a boolean value. got %s", message)
		} else if !parsed {
			r.Failf("")
		}
	}

	return r
}
