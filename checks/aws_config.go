package checks

import (
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
)

type AwsConfigChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *AwsConfigChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.AwsConfig {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

// Type: returns checker type
func (c *AwsConfigChecker) Type() string {
	return "awsconfig"
}

func (c *AwsConfigChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.AwsConfigCheck)
	result := pkg.Success(check, ctx.Canary)
	cfg, err := awsUtil.NewSession(ctx, *check.AWSConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}
	client := configservice.NewFromConfig(*cfg)
	if check.AggregatorName != nil {
		output, err := client.SelectAggregateResourceConfig(ctx, &configservice.SelectAggregateResourceConfigInput{
			ConfigurationAggregatorName: check.AggregatorName,
			Expression:                  &check.Query,
		})
		if err != nil {
			return result.ErrorMessage(err)
		}
		result.AddDetails(output.Results)
	} else {
		output, err := client.SelectResourceConfig(ctx, &configservice.SelectResourceConfigInput{
			Expression: &check.Query,
		})
		if err != nil {
			return result.ErrorMessage(err)
		}
		result.AddDetails(output.Results)
	}
	return result
}
