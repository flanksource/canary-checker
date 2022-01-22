package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true

// +kubebuilder:subresource:status
type System struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SystemSpec   `json:"spec,omitempty"`
	Status            SystemStatus `json:"status,omitempty"`
}
type SystemSpec struct {
	Type       string           `json:"type,omitempty"`
	Id         *Template        `json:"id,omitempty"`
	Tooltip    string           `json:"tooltip,omitempty"`
	Icon       string           `json:"icon,omitempty"`
	Text       string           `json:"text,omitempty"`
	Label      string           `json:"label,omitempty"`
	Owner      Owner            `json:"owner,omitempty"`
	Components []ComponentSpec  `json:"components,omitempty"`
	Canaries   []CanarySelector `json:"canaries,omitempty"`
	Properties Properties       `json:"properties,omitempty"`
}

func (system System) IsEmpty() bool {
	return len(system.Spec.Properties) == 0 && len(system.Spec.Canaries) == 0 && len(system.Spec.Components) == 0 && system.Name == ""
}

type SystemStatus struct {
	Status ComponentPropertyStatus `json:"status,omitempty"`
}

type Selector struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}
type NamespaceSelector struct {
	Selector `json:",inline"`
}

type ComponentSelector struct {
	Namespace Selector `json:"namespace,omitempty"`
	Selector  `json:",inline"`
}

type CanarySelector string
