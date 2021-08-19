package checks

import (
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons"
)

type Checker interface {
	Run(config v1.Canary) []*pkg.CheckResult
	Type() string
	Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult
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
	NewPodChecker(),
	NewNamespaceChecker(),
	NewTCPChecker(),
}

type SetsClient interface {
	SetClient(client *kommons.Client)
}
