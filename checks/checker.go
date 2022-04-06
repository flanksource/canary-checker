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
	Run(ctx *context.Context) pkg.Results
	Type() string
	Check(ctx *context.Context, extConfig external.Check) pkg.Results
}

var All = []Checker{
	&HelmChecker{},
	&DNSChecker{},
	&HTTPChecker{},
	&IcmpChecker{},
	&S3Checker{},
	&DockerPullChecker{},
	&DockerPushChecker{},
	&ContainerdPullChecker{},
	&PostgresChecker{},
	&MssqlChecker{},
	&MysqlChecker{},
	&LdapChecker{},
	&JmeterChecker{},
	&ResticChecker{},
	&RedisChecker{},
	&JunitChecker{},
	&EC2Checker{},
	&PrometheusChecker{},
	&MongoDBChecker{},
	&CloudWatchChecker{},
	&GitHubChecker{},
	&KubernetesChecker{},
	&FolderChecker{},
	&ExecChecker{},
	&AwsConfigChecker{},
	&AwsConfigRuleChecker{},
	&DatabaseBackupChecker{},
	&ConfigdbChecker{},
	NewPodChecker(),
	NewNamespaceChecker(),
	NewTCPChecker(),
}
