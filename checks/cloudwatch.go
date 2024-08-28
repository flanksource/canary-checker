//go:build !fast

package checks

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type CloudWatchChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *CloudWatchChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.CloudWatch {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *CloudWatchChecker) Type() string {
	return "cloudwatch"
}

func (c *CloudWatchChecker) Check(ctx *context.Context, check v1.CloudWatchCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if err := check.AWSConnection.Populate(ctx); err != nil {
		return results.Failf("failed to populate aws connection: %v", err)
	}

	cfg, err := check.AWSConnection.Client(ctx.Context)
	if err != nil {
		return results.ErrorMessage(err)
	}

	client := cloudwatch.NewFromConfig(cfg, func(o *cloudwatch.Options) {
		if check.AWSConnection.Endpoint != "" {
			o.BaseEndpoint = &check.AWSConnection.Endpoint
		}
	})

	maxRecords := int32(100)
	alarms, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames:      check.CloudWatchFilter.Alarms,
		AlarmNamePrefix: check.CloudWatchFilter.AlarmPrefix,
		ActionPrefix:    check.CloudWatchFilter.ActionPrefix,
		StateValue:      types.StateValue(check.CloudWatchFilter.State),
		MaxRecords:      &maxRecords,
	})
	if err != nil {
		return results.ErrorMessage(err)
	}
	if o, err := unstructure(alarms); err != nil {
		return results.ErrorMessage(err)
	} else {
		result.AddDetails(o)
	}
	firing := []string{}
	for _, alarm := range alarms.MetricAlarms {
		if alarm.StateValue == types.StateValueAlarm {
			firing = append(firing, *alarm.AlarmName)
		}
	}
	for _, alarm := range alarms.CompositeAlarms {
		if alarm.StateValue == types.StateValueAlarm {
			firing = append(firing, *alarm.AlarmName)
		}
	}
	if len(firing) > 0 {
		return results.Failf(strings.Join(firing, ","))
	}
	return results
}
