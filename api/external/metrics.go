package external

// +kubebuilder:object:generate=true
type Metrics struct {
	Name   string            `json:"name,omitempty" yaml:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Type   string            `json:"type,omitempty" yaml:"type,omitempty"`
	Value  string            `json:"value,omitempty" yaml:"value,omitempty"`
}
