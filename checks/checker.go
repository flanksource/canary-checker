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

type SingleCheckRunner interface {
	Check(ctx *context.Context, check external.Check) pkg.Results
}

var All = []Checker{
	&AlertManagerChecker{},
	&ArgoChecker{},
	&AwsConfigChecker{},
	&AwsConfigRuleChecker{},
	&AzureDevopsChecker{},
	&CloudWatchChecker{},
	&CatalogChecker{},
	&DatabaseBackupChecker{},
	&DNSChecker{},
	&DynatraceChecker{},
	&ElasticsearchChecker{},
	&ExecChecker{},
	&FolderChecker{},
	&GitHubChecker{},
	&GitProtocolChecker{},
	&HTTPChecker{},
	&IcmpChecker{},
	&JmeterChecker{},
	&JunitChecker{},
	&KubernetesChecker{},
	&KubernetesResourceChecker{},
	&LdapChecker{},
	&MongoDBChecker{},
	&MssqlChecker{},
	&MysqlChecker{},
	&OpenSearchChecker{},
	&PostgresChecker{},
	&PrometheusChecker{},
	&PubSubChecker{},
	&RedisChecker{},
	&ResticChecker{},
	&S3Checker{},
	NewNamespaceChecker(),
	NewPodChecker(),
	NewTCPChecker(),
}
