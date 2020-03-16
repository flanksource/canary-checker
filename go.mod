module github.com/flanksource/canary-checker

go 1.12

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/aws/aws-sdk-go v1.29.5
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/flanksource/commons v1.0.2
	github.com/go-ldap/ldap/v3 v3.1.7
	github.com/jasonlvhit/gocron v0.0.0-20191228163020-98b59b546dee
	github.com/lib/pq v1.3.0
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c
	github.com/spf13/cobra v0.0.5
	gopkg.in/yaml.v2 v2.2.8

	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/yaml v1.1.0
)

replace k8s.io/client-go => k8s.io/client-go v0.17.0
