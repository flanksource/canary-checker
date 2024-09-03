package v1

import (
	"fmt"
	"time"

	"github.com/flanksource/commons/hash"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/types"
	"github.com/robfig/cron/v3"
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

type TopologyTagSelector struct {
	Tag      string                 `json:"tag"`
	Selector types.ResourceSelector `json:"selector,omitempty"`
}

func (t *TopologyTagSelector) IsEmpty() bool {
	return t.Tag == "" && t.Selector.IsEmpty()
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
	// statusExpr allows defining a cel expression to evaluate the status of a component
	// based on the summary.
	HealthExpr string `json:"healthExpr,omitempty"`
	// statusExpr allows defining a cel expression to evaluate the status of a component
	// based on the summary.
	StatusExpr string `json:"statusExpr,omitempty"`
	// Properties are created once the full component tree is created, property lookup functions
	// can return a map of coomponent name => properties to allow for bulk property lookups
	// being applied to multiple components in the tree
	Properties Properties `json:"properties,omitempty"`
	// Lookup and associate config items with this component
	Configs []types.ConfigQuery `json:"configs,omitempty"`
	// Specify the catalog tag (& optionally the tag selector) to group
	// the topology.
	GroupBy TopologyTagSelector `json:"groupBy,omitempty"`

	// Agent will push topology to specified path
	PushLocation connection.HTTPConnection `json:"push,omitempty"`
}

func (s Topology) NextRuntime() (*time.Time, error) {
	if s.Spec.Schedule != "" {
		schedule, err := cron.ParseStandard(s.Spec.Schedule)
		if err != nil {
			return nil, err
		}
		t := schedule.Next(time.Now())
		return &t, nil
	}
	return nil, nil
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
