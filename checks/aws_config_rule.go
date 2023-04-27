//go:build !fast

package checks

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
	"github.com/flanksource/canary-checker/pkg/db"
)

type AwsConfigRuleChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *AwsConfigRuleChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.AwsConfigRule {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *AwsConfigRuleChecker) Type() string {
	return "awsconfigrule"
}

func (c *AwsConfigRuleChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.AwsConfigRuleCheck)
	var results pkg.Results
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)
	if check.AWSConnection == nil {
		check.AWSConnection = &v1.AWSConnection{}
	}

	if err := check.AWSConnection.FindConnection(ctx, db.Gorm); err != nil {
		return results.Failf("failed to find connection: %w", err)
	}

	cfg, err := awsUtil.NewSession(ctx, *check.AWSConnection)
	if err != nil {
		return results.Failf("failed to create a session: %v", err)
	}
	client := configservice.NewFromConfig(*cfg)

	if err != nil {
		return results.Failf("failed to describe compliance rules: %v", err)
	}

	var complianceTypes = []types.ComplianceType{}
	for _, i := range check.ComplianceTypes {
		complianceTypes = append(complianceTypes, types.ComplianceType(i))
	}
	output, err := client.DescribeComplianceByConfigRule(ctx, &configservice.DescribeComplianceByConfigRuleInput{
		ComplianceTypes: complianceTypes,
		ConfigRuleNames: check.Rules,
	})
	if err != nil {
		return results.Failf("failed to describe compliance rules: %v", err)
	}
	var complianceResults pkg.Results
	for _, complianceRule := range output.ComplianceByConfigRules {
		if configRuleInRules(check.IgnoreRules, *complianceRule.ConfigRuleName) || complianceRule.Compliance.ComplianceType == "INSUFFICIENT_DATA" || complianceRule.Compliance.ComplianceType == "NOT_APPLICABLE" {
			continue
		}
		if complianceRule.Compliance != nil {
			var complianceResult *pkg.CheckResult
			complianceCheck := check
			complianceCheck.Description.Description = fmt.Sprintf("%s - checking compliance for config rule: %s", check.Description.Description, *complianceRule.ConfigRuleName)
			if complianceRule.Compliance.ComplianceType != "COMPLIANT" {
				complianceResult = pkg.Fail(complianceCheck, ctx.Canary)
				complianceDetailsOutput, err := client.GetComplianceDetailsByConfigRule(ctx, &configservice.GetComplianceDetailsByConfigRuleInput{
					ComplianceTypes: []types.ComplianceType{
						"NON_COMPLIANT",
					},
					ConfigRuleName: complianceRule.ConfigRuleName,
				})
				if err != nil {
					complianceResult.Failf("failed to get compliance details: %v", err)
					continue
				}
				var resources []string
				for _, result := range complianceDetailsOutput.EvaluationResults {
					if result.EvaluationResultIdentifier.EvaluationResultQualifier.ResourceId != nil {
						resources = append(resources, *result.EvaluationResultIdentifier.EvaluationResultQualifier.ResourceId)
					}
				}
				complianceResult.AddDetails(resources)
				complianceResult.ResultMessage(strings.Join(resources, ","))
			} else {
				complianceResult = pkg.Success(complianceCheck, ctx.Canary)
				complianceResult.AddDetails(complianceRule)
				complianceResult.ResultMessage(fmt.Sprintf("%s rule is %v", *complianceRule.ConfigRuleName, complianceRule.Compliance.ComplianceType))
			}
			complianceResults = append(complianceResults, complianceResult)
		}
	}
	return complianceResults
}

func configRuleInRules(rules []string, ruleName string) bool {
	for _, rule := range rules {
		if strings.EqualFold(rule, ruleName) {
			return true
		}
	}
	return false
}
