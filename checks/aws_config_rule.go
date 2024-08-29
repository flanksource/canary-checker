//go:build !fast

package checks

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/connection"
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
		check.AWSConnection = &connection.AWSConnection{}
	} else if err := check.AWSConnection.Populate(ctx); err != nil {
		return results.Failf("failed to populate aws connection: %v", err)
	}

	cfg, err := check.AWSConnection.Client(ctx.Context)
	if err != nil {
		return results.Failf("failed to create a session: %v", err)
	}

	client := configservice.NewFromConfig(cfg, func(o *configservice.Options) {
		if check.AWSConnection.Endpoint != "" {
			o.BaseEndpoint = &check.AWSConnection.Endpoint
		}
	})

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

	type ConfigRuleResource struct {
		ID         string    `json:"id"`
		Annotation string    `json:"annotation"`
		Type       string    `json:"type"`
		Recorded   time.Time `json:"recorded"`
		Mode       string    `json:"mode"`
	}

	type ComplianceResult struct {

		// Supplementary information about how the evaluation determined the compliance.
		Annotation string `json:"annotation"`

		ConfigRule string `json:"rule"`

		Description string `json:"description"`
		// Indicates whether the Amazon Web Services resource complies with the Config
		// rule that evaluated it. For the EvaluationResult data type, Config supports
		// only the COMPLIANT , NON_COMPLIANT , and NOT_APPLICABLE values. Config does not
		// support the INSUFFICIENT_DATA value for the EvaluationResult data type.
		ComplianceType string `json:"type"`

		Resources []ConfigRuleResource `json:"resources"`
	}

	var complianceResults []ComplianceResult
	var failures []string
	for _, complianceRule := range output.ComplianceByConfigRules {
		if configRuleInRules(check.IgnoreRules, *complianceRule.ConfigRuleName) || complianceRule.Compliance.ComplianceType == "INSUFFICIENT_DATA" || complianceRule.Compliance.ComplianceType == "NOT_APPLICABLE" {
			continue
		}

		if complianceRule.Compliance == nil {
			continue
		}
		var data = ComplianceResult{
			ConfigRule:     *complianceRule.ConfigRuleName,
			ComplianceType: string(complianceRule.Compliance.ComplianceType),
		}

		if complianceRule.Compliance.ComplianceType != "COMPLIANT" {
			failures = append(failures, *complianceRule.ConfigRuleName)
			complianceDetailsOutput, err := client.GetComplianceDetailsByConfigRule(ctx, &configservice.GetComplianceDetailsByConfigRuleInput{
				ComplianceTypes: []types.ComplianceType{
					"NON_COMPLIANT",
				},
				ConfigRuleName: complianceRule.ConfigRuleName,
			})
			if err != nil {
				result.Failf("failed to get compliance details: %v", err)
				continue
			}
			for _, result := range complianceDetailsOutput.EvaluationResults {
				id := *result.EvaluationResultIdentifier.EvaluationResultQualifier
				data.Resources = append(data.Resources, ConfigRuleResource{
					ID:         *id.ResourceId,
					Type:       *id.ResourceType,
					Mode:       string(id.EvaluationMode),
					Recorded:   *result.ResultRecordedTime,
					Annotation: *result.Annotation,
				})
			}
		}
		complianceResults = append(complianceResults, data)
	}

	if check.Test.IsEmpty() && len(failures) > 0 {
		result.Failf(strings.Join(failures, ", "))
	}
	if r, err := unstructure(map[string]interface{}{"rules": complianceResults}); err != nil {
		result.Failf(err.Error())
	} else {
		result.AddDetails(r)
	}
	return results
}

func configRuleInRules(rules []string, ruleName string) bool {
	for _, rule := range rules {
		if strings.EqualFold(rule, ruleName) {
			return true
		}
	}
	return false
}
