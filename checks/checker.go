package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg"
)

type Checks []external.Check

func (c Checks) Includes(checker Checker) bool {
	for _, check := range c {
		if check.GetType() == checker.Type() {
			return true
		}
	}
	return false
}

type Checker interface {
	Run(ctx *context.Context) []*pkg.CheckResult
	Type() string
	Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult
}

var All = []Checker{
	&HelmChecker{},
	&DNSChecker{},
	&HTTPChecker{},
	&IcmpChecker{},
	&S3Checker{},
	&S3BucketChecker{},
	&DockerPullChecker{},
	&DockerPushChecker{},
	&ContainerdPullChecker{},
	&PostgresChecker{},
	&MssqlChecker{},
	&LdapChecker{},
	&JmeterChecker{},
	&ResticChecker{},
	&RedisChecker{},
	&JunitChecker{},
	&SmbChecker{},
	&EC2Checker{},
	&PrometheusChecker{},
	&MongoDBChecker{},
	NewPodChecker(),
	NewNamespaceChecker(),
	NewTCPChecker(),
}
