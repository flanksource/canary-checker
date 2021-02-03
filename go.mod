module github.com/flanksource/canary-checker

go 1.13

require (
	github.com/asecurityteam/rolling v2.0.4+incompatible
	github.com/aws/aws-sdk-go v1.29.25
	github.com/chartmuseum/helm-push v0.8.1
	github.com/containerd/cgroups v0.0.0-20200824123100-0b889c03f102 // indirect
	github.com/containerd/containerd v1.4.0
	github.com/docker/docker v1.13.1
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/flanksource/commons v1.4.0
	github.com/go-co-op/gocron v0.2.0
	github.com/go-ldap/ldap/v3 v3.1.7
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/hashicorp/vault/api v1.0.4
	github.com/lib/pq v1.3.0
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/mjibson/esc v0.2.0 // indirect
	github.com/ncw/swift v1.0.50
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/prometheus/common v0.7.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/cors v1.7.0
	github.com/sirupsen/logrus v1.6.0
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c
	github.com/spf13/cobra v0.0.5
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	golang.org/x/tools v0.1.0 // indirect
	gopkg.in/flanksource/yaml.v3 v3.1.1
	helm.sh/helm/v3 v3.1.2
	k8s.io/api v0.17.7
	k8s.io/apimachinery v0.17.7
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.7
	sigs.k8s.io/controller-tools v0.2.5 // indirect
	sigs.k8s.io/yaml v1.1.0
)

replace (
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
	gopkg.in/hairyhenderson/yaml.v2 => github.com/maxaudron/yaml v0.0.0-20190411130442-27c13492fe3c
	k8s.io/client-go => k8s.io/client-go v0.17.7
)
