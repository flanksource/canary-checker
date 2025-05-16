package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/shell"
)

type ExecChecker struct {
}

func (c *ExecChecker) Type() string {
	return "exec"
}

func (c *ExecChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Exec {
		results = append(results, c.Check(ctx, conf)...)
	}

	return results
}

func (c *ExecChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ExecCheck)
	result := pkg.Success(check, ctx.Canary).AddDetails(shell.ExecDetails{ExitCode: -1})

	details, err := shell.Run(ctx.Context, shell.Exec{
		Script:      check.Script,
		Connections: check.Connections,
		Checkout:    check.Checkout,
		EnvVars:     check.EnvVars,
		Artifacts:   check.Artifacts,
	})
	if err != nil {
		return result.ErrorMessage(err).Invalidf("%v", err.Error())
	}
	if details != nil {
		result.AddDetails(details)
		result.Artifacts = append(result.Artifacts, details.Artifacts...)

		if details.ExitCode != 0 {
			return result.Failf("%s", details.String()).ToSlice()
		}
	}

	return result.ToSlice()
}
