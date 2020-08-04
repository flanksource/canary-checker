package checks

import (
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type Checker interface {
	Run(config v1.CanarySpec) []*pkg.CheckResult
	Type() string
	Check(extConfig external.Check) *pkg.CheckResult
}

var All = []Checker{
	&HelmChecker{},
	&DNSChecker{},
	&HttpChecker{},
	&IcmpChecker{},
	&S3Checker{},
	&S3BucketChecker{},
	&DockerPullChecker{},
	&DockerPushChecker{},
	&PostgresChecker{},
	&LdapChecker{},
	NewPodChecker(),
	NewNamespaceChecker(),
	NewTCPChecker(),
}
