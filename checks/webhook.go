package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg"
)

type WebhookChecker struct{}

const WebhookCheckType = "webhook"

// Run returns the check spec as it is.
func (c *WebhookChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	check := ctx.Canary.Spec.Webhook
	results = append(results, pkg.Success(check, ctx.Canary))
	return results
}

func (c *WebhookChecker) Type() string {
	return WebhookCheckType
}
