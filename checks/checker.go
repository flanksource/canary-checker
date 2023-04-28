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
}

var All = []Checker{
	&AlertManagerChecker{},
	&AwsConfigChecker{},
	&AwsConfigRuleChecker{},
	&CloudWatchChecker{},
	&ConfigdbChecker{},
	&DatabaseBackupChecker{},
	&DNSChecker{},
	&EC2Checker{},
	&ElasticsearchChecker{},
	&ExecChecker{},
	&FolderChecker{},
	&GitHubChecker{},
	&HTTPChecker{},
	&IcmpChecker{},
	&JmeterChecker{},
	&JunitChecker{},
	&KubernetesChecker{},
	&LdapChecker{},
	&MongoDBChecker{},
	&MssqlChecker{},
	&MysqlChecker{},
	&PostgresChecker{},
	&PrometheusChecker{},
	&RedisChecker{},
	&ResticChecker{},
	&S3Checker{},
	NewNamespaceChecker(),
	NewPodChecker(),
	NewTCPChecker(),
}
