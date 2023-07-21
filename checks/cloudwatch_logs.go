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

	// Perform CloudWatch Logs search
	searchResults, err := c.searchLogs(ctx, client, *logGroupName, *check.Filter.Query, *check.Filter.Limit, *check.Filter.LogStreamNamePrefix, *check.Filter.StartTime, *check.Filter.EndTime)
	if err != nil {
		return results.ErrorMessage(err)
	}

	if len(searchResults) > 0 {
		return results.Failf(strings.Join(searchResults, ","))
	}

	return results
}

// searchLogs performs the CloudWatch Logs search and returns the log stream names with matching results
func (c *CloudWatchLogsChecker) searchLogs(ctx *context.Context, client *cloudwatchlogs.Client, logGroupName string, query string, limit int32, logstreamnameprefix string, startTime int64, endTime int64) ([]string, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &logGroupName,
		StartTime:           &startTime,
		Limit:               &limit,
		LogStreamNamePrefix: &logstreamnameprefix,
		EndTime:             &endTime,
		FilterPattern:       &query,
	}

	searchResults := []string{}
	paginator := cloudwatchlogs.NewFilterLogEventsPaginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, event := range page.Events {
			if event.Message != nil {
				searchResults = append(searchResults, *event.LogStreamName)
			}
		}
	}

	return searchResults, nil
}
