package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg"
)

const removedMessage = "this check type has been removed, use kubernetesResource or exec checks instead"

type removedChecker struct {
	typeName string
	specFn   func(*context.Context) []external.Check
}

func (c *removedChecker) Type() string { return c.typeName }

func (c *removedChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range c.specFn(ctx) {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *removedChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	result := pkg.Success(extConfig, ctx.Canary)
	return pkg.Results{result}.Failf(removedMessage)
}

func toChecks[T external.Check](items []T) []external.Check {
	out := make([]external.Check, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}
