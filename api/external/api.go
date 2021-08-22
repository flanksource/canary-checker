package external

// +kubebuilder:skip

type Endpointer interface {
	GetEndpoint() string
}

type Describable interface {
	GetDescription() string
	GetIcon() string
}

type WithType interface {
	GetType() string
}

type Check interface {
	Endpointer
	Describable
	WithType
}

type Template struct {
	Template string `yaml:"template,omitempty" json:"template,omitempty"`
	JSONPath string `yaml:"jsonPath,omitempty" json:"jsonPath,omitempty"`
}

type DisplayTemplate interface {
	GetDisplayTemplate() Template
}

type TestFunction interface {
	GetTestFunction() Template
}
