package checks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/robertkrimen/otto/underscore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	cUtils "github.com/flanksource/commons/utils"
	"github.com/robfig/cron/v3"
)

// DefaultArtifactConnection is the connection that's used to save all check artifacts.
var DefaultArtifactConnection string

func age(t time.Time) string {
	return utils.Age(time.Since(t))
}

func GetDeadline(canary v1.Canary) time.Time {
	if canary.Spec.Schedule != "" {
		schedule, err := cron.ParseStandard(canary.Spec.Schedule)
		if err != nil {
			// cron syntax errors are handled elsewhere, default to a 10 second timeout
			return time.Now().Add(10 * time.Second)
		}
		return schedule.Next(time.Now())
	}
	return time.Now().Add(time.Duration(canary.Spec.Interval) * time.Second)
}

func getNextRuntime(canary v1.Canary, lastRuntime time.Time) (*time.Time, error) {
	if canary.Spec.Schedule != "" {
		schedule, err := cron.ParseStandard(canary.Spec.Schedule)
		if err != nil {
			return nil, err
		}
		t := schedule.Next(time.Now())
		return &t, nil
	}
	t := lastRuntime.Add(time.Duration(canary.Spec.Interval) * time.Second)
	return &t, nil
}

// unstructure marshalls a struct to and from JSON to remove any type details
func unstructure(o any) (out map[string]any, err error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &out)
	return out, err
}

func template(ctx *context.Context, template v1.Template) (string, error) {
	tpl := template.Gomplate()

	if tpl.Functions == nil {
		tpl.Functions = make(map[string]any)
	}

	for k, v := range ctx.GetContextualFunctions() {
		tpl.Functions[k] = v
	}

	return ctx.RunTemplate(tpl, ctx.Environment)
}

func getDefaultTransformer(check external.Check) v1.Template {
	if check.GetType() == "pub_sub" && strings.HasPrefix(check.GetEndpoint(), "gcppubsub://") {
		return v1.Template{
			Expression: `dyn(results.gcp_incidents).map(r, gcp.incidents.toCheckResult(r))`,
		}
	}
	return v1.Template{}
}

// transform generates new checks from the transformation template of the parent check
func transform(ctx *context.Context, in *pkg.CheckResult) ([]*pkg.CheckResult, bool, error) {
	var tpl v1.Template
	switch v := in.Check.(type) {
	case v1.Transformer:
		tpl = v.GetTransformer()
	}

	if tpl.IsEmpty() {
		defTpl := getDefaultTransformer(in.Check)
		if defTpl.IsEmpty() {
			return []*pkg.CheckResult{in}, false, nil
		}
		tpl = defTpl
	}

	hasTransformer := true

	out, err := template(ctx, tpl)
	if err != nil {
		return nil, hasTransformer, err
	}

	var transformed []pkg.TransformedCheckResult
	if err := json.Unmarshal([]byte(out), &transformed); err != nil {
		var t pkg.TransformedCheckResult
		if errSingle := json.Unmarshal([]byte(out), &t); errSingle != nil {
			return nil, hasTransformer, err
		}
		transformed = []pkg.TransformedCheckResult{t}
	}

	var results []*pkg.CheckResult
	if len(transformed) == 0 {
		ctx.Tracef("transformation returned empty array")
		return nil, hasTransformer, nil
	}

	t := transformed[0]

	if t.Name != "" && t.Name != in.Check.GetName() {
		// new check result created with a new name
		for _, t := range transformed {
			t.Icon = cUtils.Coalesce(t.Icon, in.Check.GetIcon())
			t.Description = cUtils.Coalesce(t.Description, in.Check.GetDescription())
			t.Name = cUtils.Coalesce(t.Name, in.Check.GetName())
			t.Type = cUtils.Coalesce(t.Type, in.Check.GetType())
			t.Endpoint = cUtils.Coalesce(t.Endpoint, in.Check.GetEndpoint())
			t.TransformDeleteStrategy = cUtils.Coalesce(t.TransformDeleteStrategy, in.Check.GetTransformDeleteStrategy())

			r := t.ToCheckResult()
			r.ParentCheck = in.Check
			r.Canary = in.Canary
			r.Canary.Namespace = cUtils.Coalesce(t.Namespace, r.Canary.Namespace)
			if r.Canary.Labels == nil {
				r.Canary.Labels = make(map[string]string)
			}

			// We use this label to set the transformed column to true
			// this label are used and then removed in pkg.FromV1 function
			r.Canary.Labels["transformed"] = "true" //nolint:goconst
			if t.DeletedAt != nil && !t.DeletedAt.IsZero() {
				r.Canary.DeletionTimestamp = &metav1.Time{
					Time: *t.DeletedAt,
				}
			}

			r.Labels = t.Labels
			r.Transformed = true
			results = append(results, &r)
		}
		if ctx.IsTrace() {
			ctx.Tracef("transformed into %d results", len(results))
		}
		return results, hasTransformer, nil
	} else if len(transformed) == 1 && t.Name == "" {
		// no new checks created, in-line transformation only
		hasTransformer = false
		in.Metrics = append(in.Metrics, t.Metrics...)
		if t.Start != nil {
			in.Start = *t.Start
		}
		if t.Pass != nil {
			in.Pass = *t.Pass
		}
		if t.Invalid != nil {
			in.Invalid = *t.Invalid
		}
		if t.Duration != nil {
			in.Duration = *t.Duration
		}
		if t.Message != "" {
			in.Message = t.Message
		}
		if t.Description != "" {
			in.Description = t.Description
		}
		if t.Error != "" {
			in.Error = t.Error
		}
		if t.Detail != nil {
			if t.Detail == "$delete" {
				in.Detail = nil
				delete(in.Data, "results")
			} else {
				in.Detail = t.Detail
			}
		}
		if t.DisplayType != "" {
			in.DisplayType = t.DisplayType
		}
		if len(t.Labels) > 0 {
			in.Labels = t.Labels
		}
		if len(t.Data) > 0 {
			for k, v := range t.Data {
				in.Data[k] = v
			}
		}
	} else {
		return nil, hasTransformer, fmt.Errorf("transformation returned more than 1 entry without a name")
	}

	return []*pkg.CheckResult{in}, hasTransformer, nil
}

func GetJunitReportFromResults(canaryName string, results []*pkg.CheckResult) JunitTestSuite {
	var testSuite = JunitTestSuite{
		Name: canaryName,
	}
	for _, result := range results {
		var test JunitTest
		test.Classname = result.Check.GetType()
		test.Name = result.Check.GetDescription()
		test.Message = result.Message
		test.Duration = float64(result.Duration) / 1000
		test.Properties = result.Labels
		testSuite.Duration += float64(result.Duration) / 1000
		if result.Pass {
			testSuite.Passed++
			test.Status = "passed"
		} else {
			testSuite.Failed++
			test.Status = "failed"
			test.Error = fmt.Errorf("%s", result.Error)
		}
		testSuite.Duration += float64(result.Duration) / 1000
		testSuite.Tests = append(testSuite.Tests, test)
	}
	return testSuite
}
