package v1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// +kubebuilder:subresource:status
type SystemTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SystemTemplateSpec   `json:"spec,omitempty"`
	Status            SystemTemplateStatus `json:"status,omitempty"`
}
type SystemTemplateSpec struct {
	Type       string          `json:"type,omitempty"`
	Id         *Template       `json:"id,omitempty"` //nolint
	Schedule   string          `json:"schedule,omitempty"`
	Tooltip    string          `json:"tooltip,omitempty"`
	Icon       string          `json:"icon,omitempty"`
	Text       string          `json:"text,omitempty"`
	Label      string          `json:"label,omitempty"`
	Owner      Owner           `json:"owner,omitempty"`
	Components []ComponentSpec `json:"components,omitempty"`
	// Properties are created once the full component tree is created, property lookup functions
	// can return a map of coomponent name => properties to allow for bulk property lookups
	// being applied to multiple components in the tree
	Properties Properties `json:"properties,omitempty"`
	// Lookup and associate config items with this component
	Configs []Config `json:"configs,omitempty"`
}

func (s SystemTemplate) IsEmpty() bool {
	return len(s.Spec.Properties) == 0 && len(s.Spec.Components) == 0 && s.Name == ""
}

func (spec SystemTemplateSpec) GetSchedule() string {
	return spec.Schedule
}

type SystemTemplateStatus struct {
	PersistedID *string `json:"persistentID,omitempty"`
	// +optional
	ObservedGeneration int64  `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
	Status             string `json:"status,omitempty"`
}

func (s SystemTemplate) GetPersistedID() string {
	if s.Status.PersistedID != nil {
		return *s.Status.PersistedID
	}
	return ""
}

type Selector struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type NamespaceSelector struct {
	Selector `json:",inline"`
}

type ComponentCheck struct {
	Selector ResourceSelector `json:"selector,omitempty"`
	Inline   *CanarySpec      `json:"inline,omitempty"`
}

type Config struct {
	ID        []string          `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

func (c Config) String() string {
	s := c.Type
	if c.Namespace != "" {
		s += "/" + c.Namespace
	}

	if c.Name != "" {
		s += "/" + c.Name
	}
	if len(c.Tags) > 0 {
		s += " " + fmt.Sprintf("%v", c.Tags)
	}
	return s
}

// +kubebuilder:object:root=true

// SystemTemplateList contains a list of SystemTemplate
type SystemTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SystemTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SystemTemplate{}, &SystemTemplateList{})
}
