module github.com/flanksource/canary-checker

go 1.16

require (
	cloud.google.com/go v0.93.3 // indirect
	cloud.google.com/go/storage v1.16.0
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/allegro/bigcache v1.2.1
	github.com/asecurityteam/rolling v2.0.4+incompatible
	github.com/aws/aws-sdk-go v1.29.25
	github.com/aws/aws-sdk-go-v2 v1.8.1
	github.com/aws/aws-sdk-go-v2/config v1.5.0
	github.com/aws/aws-sdk-go-v2/credentials v1.3.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.12.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.13.0
	github.com/aws/aws-sdk-go-v2/service/ssm v1.8.1
	github.com/c2h5oh/datasize v0.0.0-20200825124411-48ed595a09d2
	github.com/chartmuseum/helm-push v0.8.1
	github.com/containerd/cgroups v0.0.0-20200824123100-0b889c03f102 // indirect
	github.com/containerd/containerd v1.4.0
	github.com/denisenkom/go-mssqldb v0.9.0
	github.com/docker/docker v1.13.1
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/eko/gocache v1.2.0
	github.com/flanksource/commons v1.5.8
	github.com/flanksource/kommons v0.25.0
	github.com/go-ldap/ldap/v3 v3.1.7
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.2.0
	github.com/go-redis/redis/v8 v8.8.2
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/hairyhenderson/gomplate/v3 v3.6.0
	github.com/hashicorp/vault/api v1.0.4
	github.com/henvic/httpretty v0.0.6
	github.com/hirochachacha/go-smb2 v1.0.10
	github.com/joshdk/go-junit v0.0.0-20210226021600-6145f504ca0d
	github.com/jszwec/csvutil v1.5.0
	github.com/lib/pq v1.9.0
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/phf/go-queue v0.0.0-20170504031614-9abe38d0371d
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.18.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.7.0
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/vadimi/go-http-ntlm v1.0.3
	github.com/vadimi/go-http-ntlm/v2 v2.4.1
	go.mongodb.org/mongo-driver v1.7.1
	golang.org/x/mod v0.5.0 // indirect
	golang.org/x/net v0.0.0-20210813160813-60bc85c4be6d
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210823070655-63515b42dcdf // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/api v0.54.0
	google.golang.org/genproto v0.0.0-20210821163610-241b8fcbd6c8 // indirect
	gopkg.in/flanksource/yaml.v3 v3.1.1
	helm.sh/helm/v3 v3.1.2
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v11.0.0+incompatible
	maze.io/x/duration v0.0.0-20160924141736-faac084b6075
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20210128214336-420b1d36250f+incompatible
	google.golang.org/grpc => google.golang.org/grpc v1.29.1
	helm.sh/helm/v3 => helm.sh/helm/v3 v3.5.1
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
	k8s.io/kubectl => k8s.io/kubectl v0.19.4
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.7.2
)
