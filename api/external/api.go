package external

import "github.com/google/uuid"

type Endpointer interface {
	GetEndpoint() string
}

type Describable interface {
	GetDescription() string
	GetIcon() string
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
	GetTransformDeleteStrategy() string
	GetMetricsSpec() []Metrics
	GetCustomUUID() uuid.UUID
	GetHash() string
	ShouldMarkFailOnEmpty() bool
	GetDependsOn() []string
}

type WithType interface {
	GetType() string
}

type Check interface {
	Endpointer
	Describable
	WithType
}
