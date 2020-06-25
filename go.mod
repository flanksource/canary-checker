module github.com/flanksource/canary-checker

go 1.12

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/aws/aws-sdk-go v1.29.25
	github.com/chartmuseum/helm-push v0.8.1
	github.com/docker/docker v1.13.1
	github.com/flanksource/commons v1.0.2
	github.com/flanksource/yaml v0.0.0-20200322131016-b7b2608b8702
	github.com/go-co-op/gocron v0.2.0
	github.com/go-ldap/ldap/v3 v3.1.7
	github.com/lib/pq v1.3.0
	github.com/ncw/swift v1.0.50
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c
	github.com/spf13/cobra v0.0.5
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	helm.sh/helm/v3 v3.1.2
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/yaml v1.1.0
)

replace (
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
	gopkg.in/hairyhenderson/yaml.v2 => github.com/maxaudron/yaml v0.0.0-20190411130442-27c13492fe3c
	k8s.io/client-go => k8s.io/client-go v0.17.0
)
