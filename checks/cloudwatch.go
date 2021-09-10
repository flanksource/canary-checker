package checks

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
)

type CloudWatchChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *CloudWatchChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.CloudWatch {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// Type: returns checker type
func (c *CloudWatchChecker) Type() string {
	return "cloudwatch"
}

func (c *CloudWatchChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.CloudWatchCheck)
	result := pkg.Success(check)
	cfg, err := awsUtil.NewSession(ctx, check.AWSConnection)
	if err != nil {
		return result.ErrorMessage(err)
	}
	client := cloudwatch.NewFromConfig(*cfg)
	maxRecords := int32(100)
	alarms, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames:      check.Filter.Alarms,
		AlarmNamePrefix: check.Filter.AlarmPrefix,
		ActionPrefix:    check.Filter.ActionPrefix,
		StateValue:      types.StateValue(check.Filter.State),
		MaxRecords:      &maxRecords,
	})
	if err != nil {
		return result.ErrorMessage(err)
	}
	result.AddDetails(alarms)
	message := ""
	for _, alarm := range alarms.MetricAlarms {
		if alarm.StateValue == types.StateValueAlarm {
			message += fmt.Sprintf("alarm '%s': is in %s state Reason: %s ReasonData: %s\n", *alarm.AlarmName, alarm.StateValue, *alarm.StateReason, *alarm.StateReasonData)
		}
	}
	for _, alarm := range alarms.CompositeAlarms {
		if alarm.StateValue == types.StateValueAlarm {
			message += fmt.Sprintf("alarm '%s': is in %s state Reason: %s ReasonData: %s\n", *alarm.AlarmName, alarm.StateValue, *alarm.StateReason, *alarm.StateReasonData)
		}
	}
	if message != "" {
		return result.Failf(message)
	}
	return result
}
