package v1

import (
	"fmt"

	"github.com/flanksource/commons/hash"
	"github.com/flanksource/duty/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// +kubebuilder:subresource:status
type Topology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TopologySpec   `json:"spec,omitempty"`
	Status            TopologyStatus `json:"status,omitempty"`
}
type TopologySpec struct {
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
	Configs []types.ConfigQuery `json:"configs,omitempty"`
}

func (s Topology) String() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}
func (s Topology) IsEmpty() bool {
	return len(s.Spec.Properties) == 0 && len(s.Spec.Components) == 0 && s.Name == ""
}

func (spec TopologySpec) GetSchedule() string {
	return spec.Schedule
}

type TopologyStatus struct {
	PersistedID *string `json:"persistentID,omitempty"`
	// +optional
	ObservedGeneration int64  `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
	Status             string `json:"status,omitempty"`
}

func (s Topology) GetPersistedID() string {
	return string(s.GetUID())
}

type Selector struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type NamespaceSelector struct {
	Selector `json:",inline"`
}

type ComponentCheck struct {
	Selector types.ResourceSelector `json:"selector,omitempty"`
	// +kubebuilder:validation:XPreserveUnknownFields
	Inline *CanarySpec `json:"inline,omitempty"`
}

func (c ComponentCheck) Hash() string {
	h, _ := hash.JSONMD5Hash(c)
	return h
}

type Config struct {
	ID        []string          `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

func (c Config) ToDuty() types.ConfigQuery {
	return types.ConfigQuery{
		ID:        c.ID,
		Type:      c.Type,
		Name:      c.Name,
		Namespace: c.Namespace,
		Tags:      c.Tags,
	}
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

// TopologyList contains a list of Topology
type TopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Topology `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Topology{}, &TopologyList{})
}
