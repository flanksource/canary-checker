module github.com/flanksource/canary-checker

go 1.16

require (
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/asecurityteam/rolling v2.0.4+incompatible
	github.com/aws/aws-sdk-go v1.29.25
	github.com/chartmuseum/helm-push v0.8.1
	github.com/containerd/cgroups v0.0.0-20200824123100-0b889c03f102 // indirect
	github.com/containerd/containerd v1.4.0
	github.com/denisenkom/go-mssqldb v0.9.0
	github.com/docker/docker v1.13.1
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/flanksource/commons v1.5.1
	github.com/flanksource/kommons v0.7.1
	github.com/go-co-op/gocron v0.2.0
	github.com/go-ldap/ldap/v3 v3.1.7
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.2.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/hashicorp/vault/api v1.0.4
	github.com/lib/pq v1.9.0
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/ncw/swift v1.0.50
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.10.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.7.0
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c
	github.com/spf13/cobra v1.1.1
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	golang.org/x/tools v0.1.0 // indirect
	gopkg.in/flanksource/yaml.v3 v3.1.1
	helm.sh/helm/v3 v3.1.2
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v11.0.0+incompatible
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.5.7
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20210128214336-420b1d36250f+incompatible

	helm.sh/helm/v3 => helm.sh/helm/v3 v3.5.1
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
	k8s.io/kubectl => k8s.io/kubectl v0.19.4
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.7.2
)
