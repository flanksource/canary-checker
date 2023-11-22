package checks

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/hash"
	"github.com/flanksource/commons/logger"
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

func RunChecks(ctx *context.Context) ([]*pkg.CheckResult, error) {
	var results []*pkg.CheckResult

	disabledChecks, err := getDisabledChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting disabled checks: %v", err)
	}

	// Check if canary is not marked deleted in DB
	if db.Gorm != nil && ctx.Canary.GetPersistedID() != "" {
		var deletedAt sql.NullTime
		err := db.Gorm.Table("canaries").Select("deleted_at").Where("id = ?", ctx.Canary.GetPersistedID()).Scan(&deletedAt).Error
		if err != nil {
			return nil, fmt.Errorf("error getting canary: %v", err)
		}

		if deletedAt.Valid {
			return results, nil
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
		logger.Errorf("error saving artifacts: %v", err)
	}

	return ProcessResults(ctx, results), nil
}

func saveArtifacts(ctx *context.Context, results pkg.Results) error {
	if DefaultArtifactConnection == "" {
		return nil
	}

	connection, err := ctx.HydrateConnectionByURL(DefaultArtifactConnection)
	if err != nil {
		return fmt.Errorf("error getting connection(%s): %w", DefaultArtifactConnection, err)
	} else if connection == nil {
		return fmt.Errorf("connection(%s) was not found", DefaultArtifactConnection)
	}

	fs, err := GetFSForConnection(utils.Ptr(ctx.Duty()), *connection)
	if err != nil {
		return fmt.Errorf("error getting filesystem for connection: %w", err)
	}
	defer fs.Close()

	for _, r := range results {
		if len(r.Artifacts) == 0 {
			continue
		}

		for _, a := range r.Artifacts {
			info, err := fs.Write(ctx, a.Path, a.Content)
			if err != nil {
				logger.Errorf("error saving artifact to filestore: %v", err)
				continue
			}

			checkIDRaw := ctx.Canary.Status.Checks[r.Check.GetName()]
			checkID, err := uuid.Parse(checkIDRaw)
			if err != nil {
				logger.Errorf("error parsing checkID(%s): %v", checkIDRaw, err)
				continue
			}

			artifact := models.Artifact{
				CheckID:      utils.Ptr(checkID),
				CheckTime:    utils.Ptr(r.Start),
				ConnectionID: connection.ID,
				Path:         a.Path,
				Filename:     info.Name(),
				Size:         info.Size(),
				ContentType:  a.ContentType,
				Checksum:     hash.Sha256Hex(string(a.Content)),
			}

			if err := ctx.DB().Create(&artifact).Error; err != nil {
				logger.Errorf("error saving artifact to db: %v", err)
			}
		}
	}

	return nil
}

func TransformResults(ctx *context.Context, in []*pkg.CheckResult) (out []*pkg.CheckResult) {
	for _, r := range in {
		checkCtx := ctx.WithCheckResult(r)
		transformed, err := transform(checkCtx, r)
		if err != nil {
			r.Failf("transformation failure: %v", err)
			out = append(out, r)
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
		logger.Errorf("Unknown result mode: %s", ctx.Canary.Spec.ResultMode)
	}

	return results
}

func processTemplates(ctx *context.Context, r *pkg.CheckResult) *pkg.CheckResult {
	if r.Duration == 0 && r.GetDuration() > 0 {
		r.Duration = r.GetDuration()
	}
	switch v := r.Check.(type) {
	case v1.DisplayTemplate:
		if !v.GetDisplayTemplate().IsEmpty() {
			message, err := template(ctx, v.GetDisplayTemplate())
			if err != nil {
				r.ErrorMessage(err)
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
			r.ErrorMessage(err)
		} else if message != "true" {
			if message != "false" {
				r.Failf("expecting either 'true' or 'false' but got '%v'", message)
			} else {
				r.Failf("")
			}
		}
	}

	return r
}
