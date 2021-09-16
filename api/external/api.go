package external

// +kubebuilder:skip

type Endpointer interface {
	GetEndpoint() string
}

type Describable interface {
	GetDescription() string
	GetIcon() string
	GetName() string
}

type WithType interface {
	GetType() string
}

type Check interface {
	Endpointer
	Describable
	WithType
}
