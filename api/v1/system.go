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
	Type    string            `json:"type,omitempty"`
	Id      *Template         `json:"id,omitempty"` //nolint
	Tooltip string            `json:"tooltip,omitempty"`
	Icon    string            `json:"icon,omitempty"`
	Text    string            `json:"text,omitempty"`
	Label   string            `json:"label,omitempty"`
	Owner   Owner             `json:"owner,omitempty"`
	Pods    map[string]string `json:"pods,omitempty"`
	// ComponentSelector []ComponentSelector `json:"componentSelector,omitempty"`
	Components []ComponentSpec  `json:"components,omitempty"`
	Canaries   []CanarySelector `json:"canaries,omitempty"`
	Properties Properties       `json:"properties,omitempty"`
}

func (system System) IsEmpty() bool {
	return len(system.Spec.Properties) == 0 && len(system.Spec.Canaries) == 0 && len(system.Spec.Components) == 0 && system.Name == ""
}

type ComponentSelector struct {
	Selector   `json:",inline"`
	Properties map[string]string `json:"properties,omitempty"`
}

type SystemStatus struct {
	Status string `json:"status,omitempty"`
}

type Selector struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}
type NamespaceSelector struct {
	Selector `json:",inline"`
}

type CanarySelector string
