package checks

import (
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons"
)

type Checker interface {
	Run(config v1.CanarySpec) []*pkg.CheckResult
	Type() string
	Check(extConfig external.Check) *pkg.CheckResult
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
	NewPodChecker(),
	NewNamespaceChecker(),
	NewTCPChecker(),
}

type SetsClient interface {
	SetClient(client *kommons.Client)
}
