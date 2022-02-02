package checks

import (
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
)

type AwsConfigRuleChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *AwsConfigRuleChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.AwsConfigRule {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

// Type: returns checker type
func (c *AwsConfigRuleChecker) Type() string {
	return "awsconfigrule"
}

func (c *AwsConfigRuleChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.AwsConfigRuleCheck)
	result := pkg.Success(check, ctx.Canary)
	if check.AWSConnection == nil {
		check.AWSConnection = &v1.AWSConnection{}
	}
	cfg, err := awsUtil.NewSession(ctx, *check.AWSConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}
	client := configservice.NewFromConfig(*cfg)

	output, err := client.DescribeComplianceByConfigRule(ctx, &configservice.DescribeComplianceByConfigRuleInput{
		ComplianceTypes: check.ComplianceTypes,
		ConfigRuleNames: check.Rules,
	})
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(output.ComplianceByConfigRules)
	return result
}
