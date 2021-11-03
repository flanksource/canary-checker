package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons/ktemplate"
	"github.com/ghodss/yaml"
	"reflect"
)

type Checks []external.Check

func Contextualise(check external.Check, ctx *context.Context) (external.Check, error) {
	// Function to merge metadata from environment/defaulting/chained checks into check structure
	updated := reflect.Zero(reflect.TypeOf(check)).Elem().Interface()

	checkText, err := yaml.Marshal(check)
	if err != nil {
		return check, err
	}
	defaultText, err := yaml.Marshal(ctx.Canary.Spec.Defaults)
	if err != nil {
		return check, err
	}
	err = yaml.Unmarshal(defaultText, &updated)
	if err != nil {
		return check, err
	}
	err = yaml.Unmarshal(checkText, &updated)
	if err != nil {
		return check, err
	}
	client, err := ctx.Kommons.GetClientset()
	if err != nil {
		return check, err
	}
	templater := ktemplate.StructTemplater{
		Values: ctx.Environment,
		Clientset: client,
	}
	err = templater.Walk(&updated)
	if err != nil {
		return check, nil
	}
	return updated.(external.Check), nil
}

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
	&EC2Checker{},
	&PrometheusChecker{},
	&MongoDBChecker{},
	&CloudWatchChecker{},
	&GitHubChecker{},
	&KubernetesChecker{},
	&FolderChecker{},
	&ExecChecker{},
	NewPodChecker(),
	NewNamespaceChecker(),
	NewTCPChecker(),
}
