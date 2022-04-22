package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true

// +kubebuilder:subresource:status
type SystemTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SystemTemplateSpec   `json:"spec,omitempty"`
	Status            SystemTemplateStatus `json:"status,omitempty"`
}
type SystemTemplateSpec struct {
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

func (system SystemTemplate) IsEmpty() bool {
	return len(system.Spec.Properties) == 0 && len(system.Spec.Canaries) == 0 && len(system.Spec.Components) == 0 && system.Name == ""
}

type ComponentSelector struct {
	Selector   `json:",inline"`
	Properties map[string]string `json:"properties,omitempty"`
}

type SystemTemplateStatus struct {
	PersistentID *string `json:"persistentID,omitempty"`
	// +optional
	ObservedGeneration int64  `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
	Status             string `json:"status,omitempty"`
}

type Selector struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}
type NamespaceSelector struct {
	Selector `json:",inline"`
}

type CanarySelector string

// +kubebuilder:object:root=true

// SystemTemplateList contains a list of Canary
type SystemTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SystemTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SystemTemplate{}, &SystemTemplateList{})
}
