package checks

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	awsUtil "github.com/flanksource/canary-checker/pkg/clients/aws"
)

type CloudWatchLogsChecker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *CloudWatchLogsChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.CloudWatchLogs {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *CloudWatchLogsChecker) Type() string {
	return "cloudwatchlogs"
}

func (c *CloudWatchLogsChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.CloudWatchLogsCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if err := check.AWSConnection.Populate(ctx, ctx.Kubernetes, ctx.Namespace); err != nil {
		return results.Failf("failed to populate aws connection: %v", err)
	}

	cfg, err := awsUtil.NewSession(ctx, check.AWSConnection)
	if err != nil {
		return results.ErrorMessage(err)
	}
	client := cloudwatchlogs.NewFromConfig(*cfg)

	logGroupName := check.Filter.LogGroup

	input := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        logGroupName,
		Descending:          check.Filter.Descending,
		Limit:               check.Filter.Limit,
		LogStreamNamePrefix: check.Filter.LogStreamNamePrefix,
	}

	streams, err := client.DescribeLogStreams(ctx, input)
	if err != nil {
		return results.ErrorMessage(err)
	}

	failingStreams := []string{}
	for _, stream := range streams.LogStreams {
		if *stream.StoredBytes == 0 {
			failingStreams = append(failingStreams, *stream.LogStreamName)
		}
	}

	if len(failingStreams) > 0 {
		return results.Failf(strings.Join(failingStreams, ","))
	}

	return results
}
