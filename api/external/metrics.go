package external

// +kubebuilder:object:generate=true
type Metrics struct {
	Name   string       `json:"name,omitempty" yaml:"name,omitempty"`
	Labels MetricLabels `json:"labels,omitempty" yaml:"labels,omitempty"`
	Type   string       `json:"type,omitempty" yaml:"type,omitempty"`
	Value  string       `json:"value,omitempty" yaml:"value,omitempty"`
}

type MetricLabels []MetricLabel

type MetricLabel struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	ValueExpr string `json:"valueExpr,omitempty"`
}

func (labels MetricLabels) Names() []string {
	var names []string
	for _, k := range labels {
		names = append(names, k.Name)
	}
	return names
}
