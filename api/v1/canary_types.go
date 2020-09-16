/*
Copyright 2020 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"fmt"

	"github.com/flanksource/canary-checker/api/external"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CanarySpec defines the desired state of Canary
type CanarySpec struct {
	Env            map[string]VarSource  `yaml:"env,omitempty" json:"env,omitempty"`
	HTTP           []HTTPCheck           `yaml:"http,omitempty" json:"http,omitempty"`
	DNS            []DNSCheck            `yaml:"dns,omitempty" json:"dns,omitempty"`
	DockerPull     []DockerPullCheck     `yaml:"docker,omitempty" json:"docker,omitempty"`
	DockerPush     []DockerPushCheck     `yaml:"dockerPush,omitempty" json:"dockerPush,omitempty"`
	ContainerdPull []ContainerdPullCheck `yaml:"containerd,omitempty" json:"containerd,omitempty"`
	ContainerdPush []ContainerdPushCheck `yaml:"containerdPush,omitempty" json:"containerdPush,omitempty"`
	S3             []S3Check             `yaml:"s3,omitempty" json:"s3,omitempty"`
	S3Bucket       []S3BucketCheck       `yaml:"s3Bucket,omitempty" json:"s3Bucket,omitempty"`
	TCP            []TCPCheck            `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Pod            []PodCheck            `yaml:"pod,omitempty" json:"pod,omitempty"`
	LDAP           []LDAPCheck           `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	SSL            []SSLCheck            `yaml:"ssl,omitempty" json:"ssl,omitempty"`
	ICMP           []ICMPCheck           `yaml:"icmp,omitempty" json:"icmp,omitempty"`
	Postgres       []PostgresCheck       `yaml:"postgres,omitempty" json:"postgres,omitempty"`
	Helm           []HelmCheck           `yaml:"helm,omitempty" json:"helm,omitempty"`
	Namespace      []NamespaceCheck      `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Interval       uint64                `yaml:"interval,omitempty" json:"interval,omitempty"`
}

func (spec CanarySpec) GetAllChecks() []external.Check {
	var checks []external.Check
	for _, check := range spec.HTTP {
		checks = append(checks, check)
	}
	for _, check := range spec.DNS {
		checks = append(checks, check)
	}
	for _, check := range spec.DockerPull {
		checks = append(checks, check)
	}
	for _, check := range spec.DockerPush {
		checks = append(checks, check)
	}
	for _, check := range spec.ContainerdPull {
		checks = append(checks, check)
	}
	for _, check := range spec.ContainerdPush {
		checks = append(checks, check)
	}
	for _, check := range spec.S3 {
		checks = append(checks, check)
	}
	for _, check := range spec.S3Bucket {
		checks = append(checks, check)
	}
	for _, check := range spec.TCP {
		checks = append(checks, check)
	}
	for _, check := range spec.Pod {
		checks = append(checks, check)
	}
	for _, check := range spec.LDAP {
		checks = append(checks, check)
	}
	for _, check := range spec.SSL {
		checks = append(checks, check)
	}
	for _, check := range spec.Postgres {
		checks = append(checks, check)
	}
	for _, check := range spec.ICMP {
		checks = append(checks, check)
	}
	for _, check := range spec.Helm {
		checks = append(checks, check)
	}
	for _, check := range spec.Namespace {
		checks = append(checks, check)
	}

	return checks
}

func (c Canary) GetKey(check external.Check) string {
	return fmt.Sprintf("%s/%s:%s", c.ID(), check.GetType(), c.GetDescription(check))
}

func (c Canary) GetDescription(check external.Check) string {
	if check.GetDescription() != "" {
		return check.GetDescription()
	}
	return check.GetEndpoint()
}

type CanaryStatusCondition string

var (
	Passed  CanaryStatusCondition = "Passed"
	Failed  CanaryStatusCondition = "Failed"
	Invalid CanaryStatusCondition = "Invalid"
)

// CanaryStatus defines the observed state of Canary
type CanaryStatus struct {
	// +optional
	LastTransitionedTime *metav1.Time `json:"lastTransitionedTime,omitempty"`
	// +optionals
	LastCheck *metav1.Time `json:"lastCheck,omitempty"`
	// +optional
	Status *CanaryStatusCondition `json:"status,omitempty"`
	// +optional
	Message *string `json:"message,omitempty"`
	// If set, this represents the .metadata.generation that the status was set for
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`

	// Availibility over a rolling 1h period
	Uptime1H string `json:"uptime1h,omitempty"`

	// Average latency to complete all checks
	Latency1H string `json:"latency1h,omitempty"`
}

// +kubebuilder:object:root=true

// Canary is the Schema for the canaries API
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.interval`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Uptime 1H",type=string,JSONPath=`.status.uptime1h`
// +kubebuilder:printcolumn:name="Latency 1H",type=string,JSONPath=`.status.latency1h`
// +kubebuilder:printcolumn:name="Last Transitioned",type=date,JSONPath=`.status.lastTransitionedTime`
// +kubebuilder:printcolumn:name="Last Check",type=date,JSONPath=`.status.lastCheck`
// +kubebuilder:subresource:status
type Canary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanarySpec   `json:"spec,omitempty"`
	Status CanaryStatus `json:"status,omitempty"`
}

func (c Canary) ID() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

// +kubebuilder:object:root=true

// CanaryList contains a list of Canary
type CanaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Canary `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Canary{}, &CanaryList{})
}
