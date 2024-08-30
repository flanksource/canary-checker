//go:build !fast

package checks

import (
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/connection"
)

type AwsConfigChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *AwsConfigChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.AwsConfig {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *AwsConfigChecker) Type() string {
	return "awsconfig"
}

func (c *AwsConfigChecker) Check(ctx *context.Context, check v1.AwsConfigCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if check.AWSConnection == nil {
		check.AWSConnection = &connection.AWSConnection{}
	} else {
		if err := check.AWSConnection.Populate(ctx); err != nil {
			return results.Failf("failed to populate aws connection: %v", err)
		}
	}

	cfg, err := check.AWSConnection.Client(ctx.Context)
	if err != nil {
		return results.ErrorMessage(err)
	}

	client := configservice.NewFromConfig(cfg, func(o *configservice.Options) {
		if check.AWSConnection.Endpoint != "" {
			o.BaseEndpoint = &check.AWSConnection.Endpoint
		}
	})

	if check.AggregatorName != nil {
		output, err := client.SelectAggregateResourceConfig(ctx, &configservice.SelectAggregateResourceConfigInput{
			ConfigurationAggregatorName: check.AggregatorName,
			Expression:                  &check.Query,
		})
		if err != nil {
			return results.ErrorMessage(err)
		}
		result.AddDetails(output.Results)
	} else {
		output, err := client.SelectResourceConfig(ctx, &configservice.SelectResourceConfigInput{
			Expression: &check.Query,
		})
		if err != nil {
			return results.ErrorMessage(err)
		}
		result.AddDetails(output.Results)
	}

	return results
}
