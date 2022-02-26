//go:build fast

package checks

import (
	"errors"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg"
)

type S3Checker struct{}
type EC2Checker struct{}
type CloudWatchChecker struct{}
type AwsConfigChecker struct{}
type AwsConfigRuleChecker struct{}

func (c *AwsConfigRuleChecker) Type() string {
	return "awsconfigrule"
}

func (c *AwsConfigRuleChecker) Run(ctx *context.Context) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}
func (c *AwsConfigRuleChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *AwsConfigChecker) Type() string {
	return "awsconfig"
}

func (c *AwsConfigChecker) Run(ctx *context.Context) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *EC2Checker) Run(ctx *context.Context) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *CloudWatchChecker) Run(ctx *context.Context) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *S3Checker) Run(ctx *context.Context) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *AwsConfigChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *CloudWatchChecker) Type() string {
	return "cloudwatch"
}

func (c *CloudWatchChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

// Type: returns checker type
func (c *EC2Checker) Type() string {
	return "ec2"
}

func (c *EC2Checker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func (c *S3Checker) Type() string {
	return "s3"
}

func (c *S3Checker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}

func CheckS3Bucket(ctx *context.Context, extConfig external.Check) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("AWS not included in binary"))
}
