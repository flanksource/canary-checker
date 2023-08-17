package external

type Endpointer interface {
	GetEndpoint() string
}

type Describable interface {
	GetDescription() string
	GetIcon() string
	GetName() string
	GetLabels() map[string]string
	GetTransformDeleteStrategy() string
	GetMetricsSpec() Metrics
}

type WithType interface {
	GetType() string
}

type Check interface {
	Endpointer
	Describable
	WithType
}
