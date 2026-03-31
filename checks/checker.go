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
	&removedChecker{typeName: "github", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.GitHub)
	}},
	&removedChecker{typeName: "gitProtocol", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.GitProtocol)
	}},
	&removedChecker{typeName: "containerdPull", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.ContainerdPull)
	}},
	&removedChecker{typeName: "containerdPush", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.ContainerdPush)
	}},
	&removedChecker{typeName: "dockerPull", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.DockerPull)
	}},
	&removedChecker{typeName: "dockerPush", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.DockerPush)
	}},
	&removedChecker{typeName: "helm", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.Helm)
	}},
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
	&removedChecker{typeName: "namespace", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.Namespace)
	}},
	&removedChecker{typeName: "pod", specFn: func(ctx *context.Context) []external.Check {
		return toChecks(ctx.Canary.Spec.Pod)
	}},
	NewTCPChecker(),
}
