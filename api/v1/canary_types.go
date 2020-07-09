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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CanarySpec defines the desired state of Canary
type CanarySpec struct {
	Env        map[string]VarSource `yaml:"env,omitempty" json:"env,omitempty"`
	HTTP       []HTTPCheck          `yaml:"http,omitempty" json:"http,omitempty"`
	DNS        []DNSCheck           `yaml:"dns,omitempty" json:"dns,omitempty"`
	DockerPull []DockerPullCheck    `yaml:"docker,omitempty" json:"docker,omitempty"`
	DockerPush []DockerPushCheck    `yaml:"dockerPush,omitempty" json:"dockerPush,omitempty"`
	S3         []S3Check            `yaml:"s3,omitempty" json:"s3,omitempty"`
	S3Bucket   []S3BucketCheck      `yaml:"s3Bucket,omitempty" json:"s3Bucket,omitempty"`
	TCP        []TCPCheck           `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Pod        []PodCheck           `yaml:"pod,omitempty" json:"pod,omitempty"`
	LDAP       []LDAPCheck          `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	SSL        []SSLCheck           `yaml:"ssl,omitempty" json:"ssl,omitempty"`
	ICMP       []ICMPCheck          `yaml:"icmp,omitempty" json:"icmp,omitempty"`
	Postgres   []PostgresCheck      `yaml:"postgres,omitempty" json:"postgres,omitempty"`
	Helm       []HelmCheck          `yaml:"helm,omitempty" json:"helm,omitempty"`
	Namespace  []NamespaceCheck     `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Interval   int64                `json:"interval,omitempty"`
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
}

// +kubebuilder:object:root=true

// Canary is the Schema for the canaries API
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.interval`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Last Transitioned",type=date,JSONPath=`.status.lastTransitionedTime`
// +kubebuilder:printcolumn:name="Last Check",type=date,JSONPath=`.status.lastCheck`
// +kubebuilder:subresource:status
type Canary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanarySpec   `json:"spec,omitempty"`
	Status CanaryStatus `json:"status,omitempty"`
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
