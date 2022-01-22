package checks

import (
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/robertkrimen/otto/underscore"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/canary-checker/templating"
	"github.com/flanksource/kommons"
	"github.com/flanksource/kommons/ktemplate"
	"github.com/robfig/cron/v3"
)

func GetConnection(ctx *context.Context, conn *v1.Connection, namespace string) (string, error) {
	// TODO: this function should not be necessary, each check should be templated out individual
	// however, the walk method only support high level values, not values from siblings.

	if conn.Authentication.IsEmpty() {
		return conn.Connection, nil
	}
	client, err := ctx.Kommons.GetClientset()
	if err != nil {
		return "", err
	}

	auth, err := GetAuthValues(&conn.Authentication, ctx.Kommons, namespace)
	if err != nil {
		return "", err
	}

	clone := conn.DeepCopy()

	data := map[string]interface{}{
		"name":      ctx.Canary.Name,
		"namespace": namespace,
		"username":  auth.GetUsername(),
		"password":  auth.GetPassword(),
		"domain":    auth.GetDomain(),
	}
	templater := ktemplate.StructTemplater{
		Clientset: client,
		Values:    data,
		// access go values in template requires prefix everything with .
		// to support $(username) instead of $(.username) we add a function for each var
		ValueFunctions: true,
		DelimSets: []ktemplate.Delims{
			{Left: "{{", Right: "}}"},
			{Left: "$(", Right: ")"},
		},
		RequiredTag: "template",
	}
	if err := templater.Walk(clone); err != nil {
		return "", err
	}

	return clone.Connection, nil
}

func GetAuthValues(auth *v1.Authentication, client *kommons.Client, namespace string) (*v1.Authentication, error) {
	authentication := &v1.Authentication{
		Username: kommons.EnvVar{
			Value: "",
		},
		Password: kommons.EnvVar{
			Value: "",
		},
	}
	// in case nil we are sending empty string values for username and password
	if auth == nil {
		return authentication, nil
	}
	_, username, err := client.GetEnvValue(auth.Username, namespace)
	if err != nil {
		return nil, err
	}
	authentication.Username = kommons.EnvVar{
		Value: username,
	}
	_, password, err := client.GetEnvValue(auth.Password, namespace)
	if err != nil {
		return nil, err
	}
	authentication.Password = kommons.EnvVar{
		Value: password,
	}
	return authentication, err
}

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

func def(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func template(ctx *context.Context, template v1.Template) (string, error) {
	return templating.Template(ctx.Environment, template)
}

func transform(ctx *context.Context, in *pkg.CheckResult) ([]*pkg.CheckResult, error) {
	var tpl v1.Template
	switch v := in.Check.(type) {
	case v1.Transformer:
		tpl = v.GetTransformer()
	}

	if tpl.IsEmpty() {
		return []*pkg.CheckResult{in}, nil
	}

	out, err := template(ctx.New(in.Data), tpl)
	if err != nil {
		return nil, err
	}

	var transformed []pkg.TransformedCheckResult

	if err := json.Unmarshal([]byte(out), &transformed); err != nil {
		return nil, err
	}

	var results []*pkg.CheckResult

	for _, t := range transformed {
		t.Icon = def(t.Icon, in.Check.GetIcon())
		t.Description = def(t.Description, in.Check.GetDescription())
		t.Name = def(t.Name, in.Check.GetName())
		t.Type = def(t.Type, in.Check.GetType())
		t.Endpoint = def(t.Endpoint, in.Check.GetEndpoint())
		r := t.ToCheckResult()
		r.Canary = in.Canary
		r.Canary.Namespace = def(t.Namespace, r.Canary.Namespace)
		for k, v := range t.Labels {
			if r.Canary.Labels == nil {
				r.Canary.Labels = make(map[string]string)
			}
			r.Canary.Labels[k] = v
		}
		results = append(results, &r)
	}

	if ctx.IsTrace() {
		ctx.Tracef("transformed %s into %v", in, results)
	}

	return results, nil
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
		testSuite.Duration += float64(result.Duration) / 1000
		if result.Pass {
			testSuite.Passed++
			test.Status = "passed"
		} else {
			testSuite.Failed++
			test.Status = "failed"
			test.Error = fmt.Errorf(result.Error)
		}
		testSuite.Duration += float64(result.Duration) / 1000
		testSuite.Tests = append(testSuite.Tests, test)
	}
	return testSuite
}
