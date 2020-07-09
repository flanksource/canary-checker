module github.com/flanksource/canary-checker

go 1.13

require (
	github.com/Azure/azure-sdk-for-go v32.5.0+incompatible // indirect
	github.com/Azure/go-autorest v12.2.0+incompatible // indirect
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/apex/log v1.4.0
	github.com/aws/aws-sdk-go v1.29.25
	github.com/brancz/gojsontoyaml v0.0.0-20191212081931-bf2969bbd742 // indirect
	github.com/chartmuseum/helm-push v0.8.1
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v1.13.1
	github.com/flanksource/commons v1.3.6
	github.com/flanksource/yaml v0.0.0-20200322131016-b7b2608b8702 // indirect
	github.com/go-co-op/gocron v0.2.0
	github.com/go-ldap/ldap/v3 v3.1.7
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.0
	github.com/lib/pq v1.3.0
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/ncw/swift v1.0.50
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.4.2
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	go.uber.org/zap v1.15.0
	golang.org/x/build v0.0.0-20190111050920-041ab4dc3f9d // indirect
	golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	gopkg.in/flanksource/yaml.v3 v3.1.1
	helm.sh/helm/v3 v3.1.2
	k8s.io/api v0.17.7
	k8s.io/apimachinery v0.17.7
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.7
	sigs.k8s.io/yaml v1.1.0
)

replace (
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
	github.com/flanksource/commons => ../../moshloop/commons
	gopkg.in/hairyhenderson/yaml.v2 => github.com/maxaudron/yaml v0.0.0-20190411130442-27c13492fe3c
	k8s.io/client-go => k8s.io/client-go v0.17.7
)
