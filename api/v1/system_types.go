package v1

import (
	"encoding/json"
	"fmt"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:object:root=true

// +kubebuilder:subresource:status
type Topology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TopologySpec   `json:"spec,omitempty"`
	Status            TopologyStatus `json:"status,omitempty"`
}

func (t *Topology) ToModel() *models.Topology {
	spec, _ := json.Marshal(t.Spec)
	return &models.Topology{
		Name:      t.GetName(),
		Namespace: t.GetNamespace(),
		Labels:    types.JSONStringMap(t.GetLabels()),
		Spec:      spec,
	}
}

func (t Topology) IsEmpty() bool {
	return len(t.Spec.Properties) == 0 && len(t.Spec.Components) == 0 && t.Name == ""
}

func (t Topology) GetPersistedID() string {
	return string(t.GetUID())
}

func TopologyFromModels(t models.Topology) Topology {
	var topologySpec TopologySpec
	id := t.ID.String()
	if err := json.Unmarshal(t.Spec, &topologySpec); err != nil {
		logger.Errorf("error unmarshalling topology spec %s", err)
	}
	return Topology{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Topology",
			APIVersion: "canaries.flanksource.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
			Labels:    t.Labels,
			UID:       k8sTypes.UID(id),
		},
		Spec: topologySpec,
	}
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
	Configs []Config `json:"configs,omitempty"`
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

type Config struct {
	ID        []string          `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

func (c Config) ToModel() *types.ConfigQuery {
	return &types.ConfigQuery{
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

type Configs []*Config

func (c Configs) ToModel() types.ConfigQueries {
	queries := make(types.ConfigQueries, len(c))
	for i, c := range c {
		queries[i] = c.ToModel()
	}

	return queries
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
