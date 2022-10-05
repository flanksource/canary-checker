module github.com/flanksource/canary-checker/fixtures-crd/datasources

go 1.16

require (
	github.com/aws/aws-sdk-go v1.29.25
	github.com/flanksource/commons v1.5.8
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/ncw/swift v1.0.53
	github.com/pkg/errors v0.9.1
	github.com/rabbitmq/amqp091-go v1.5.0
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v11.0.0+incompatible
)

replace k8s.io/client-go => k8s.io/client-go v0.19.4
