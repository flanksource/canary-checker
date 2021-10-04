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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/commons/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mssqlDriver    = "mssql"
	postgresDriver = "postgres"
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
	GCSBucket      []GCSBucketCheck      `yaml:"gcsBucket,omitempty" json:"gcsBucket,omitempty"`
	TCP            []TCPCheck            `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Pod            []PodCheck            `yaml:"pod,omitempty" json:"pod,omitempty"`
	LDAP           []LDAPCheck           `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	ICMP           []ICMPCheck           `yaml:"icmp,omitempty" json:"icmp,omitempty"`
	Postgres       []PostgresCheck       `yaml:"postgres,omitempty" json:"postgres,omitempty"`
	Mssql          []MssqlCheck          `yaml:"mssql,omitempty" json:"mssql,omitempty"`
	Restic         []ResticCheck         `yaml:"restic,omitempty" json:"restic,omitempty"`
	Jmeter         []JmeterCheck         `yaml:"jmeter,omitempty" json:"jmeter,omitempty"`
	Junit          []JunitCheck          `yaml:"junit,omitempty" json:"junit,omitempty"`
	Smb            []SmbCheck            `yaml:"smb,omitempty" json:"smb,omitempty"`
	Helm           []HelmCheck           `yaml:"helm,omitempty" json:"helm,omitempty"`
	Namespace      []NamespaceCheck      `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Redis          []RedisCheck          `yaml:"redis,omitempty" json:"redis,omitempty"`
	EC2            []EC2Check            `yaml:"ec2,omitempty" json:"ec2,omitempty"`
	Prometheus     []PrometheusCheck     `yaml:"prometheus,omitempty" json:"prometheus,omitempty"`
	MongoDB        []MongoDBCheck        `yaml:"mongodb,omitempty" json:"mongodb,omitempty"`
	CloudWatch     []CloudWatchCheck     `yaml:"cloudwatch,omitempty" json:"cloudwatch,omitempty"`
	GitHub         []GitHubCheck         `yaml:"github,omitempty" json:"github,omitempty"`
	Kubernetes     []KubernetesCheck     `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	Folder         []FolderCheck         `yaml:"folder,omitempty" json:"folder,omitempty"`
	// interval (in seconds) to run checks on Deprecated in favor of Schedule
	Interval uint64 `yaml:"interval,omitempty" json:"interval,omitempty"`
	// Schedule to run checks on. Supports all cron expression, example: '30 3-6,20-23 * * *'. For more info about cron expression syntax see https://en.wikipedia.org/wiki/Cron
	//  Also supports golang duration, can be set as '@every 1m30s' which runs the check every 1 minute and 30 seconds.
	Schedule string `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Icon     string `yaml:"icon,omitempty" json:"icon,omitempty"`
	Severity string `yaml:"severity,omitempty" json:"severity,omitempty"`
	Owner    string `yaml:"owner,omitempty" json:"owner,omitempty"`
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
	for _, check := range spec.Postgres {
		checks = append(checks, check)
	}
	for _, check := range spec.Mssql {
		checks = append(checks, check)
	}
	for _, check := range spec.Redis {
		checks = append(checks, check)
	}
	for _, check := range spec.Restic {
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
	for _, check := range spec.Jmeter {
		checks = append(checks, check)
	}
	for _, check := range spec.Junit {
		checks = append(checks, check)
	}
	for _, check := range spec.Smb {
		checks = append(checks, check)
	}
	for _, check := range spec.EC2 {
		checks = append(checks, check)
	}
	for _, check := range spec.Prometheus {
		checks = append(checks, check)
	}
	for _, check := range spec.MongoDB {
		checks = append(checks, check)
	}
	for _, check := range spec.GCSBucket {
		checks = append(checks, check)
	}
	for _, check := range spec.CloudWatch {
		checks = append(checks, check)
	}
	for _, check := range spec.GitHub {
		checks = append(checks, check)
	}
	for _, check := range spec.Kubernetes {
		checks = append(checks, check)
	}
	for _, check := range spec.Folder {
		checks = append(checks, check)
	}
	return checks
}

func (spec CanarySpec) SetSQLDrivers() {
	for i := range spec.Mssql {
		if spec.Mssql[i].driver == "" {
			spec.Mssql[i].SetDriver(mssqlDriver)
		}
	}
	for i := range spec.Postgres {
		if spec.Postgres[i].driver == "" {
			spec.Postgres[i].SetDriver(postgresDriver)
		}
	}
}

func (c Canary) GetKey(check external.Check) string {
	data, err := json.Marshal(check)
	if err != nil {
		logger.Debugf("error marshalling the check")
	}
	var hash = md5.Sum(data)
	return fmt.Sprintf("%s/%s:%s/%s:%s", c.ID(), check.GetType(), check.GetName(), c.GetDescription(check), hex.EncodeToString(hash[:]))
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
	// +optional
	LastCheck *metav1.Time `json:"lastCheck,omitempty"`
	// +optional
	Message *string `json:"message,omitempty"`
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`
	// +optional
	Status *CanaryStatusCondition `json:"status,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
	// +optional
	ChecksStatus map[string]*CheckStatus `json:"checkStatus,omitempty"`
	// Availibility over a rolling 1h period
	Uptime1H string `json:"uptime1h,omitempty"`
	// Average latency to complete all checks
	Latency1H string `json:"latency1h,omitempty"`
}

type CheckStatus struct {
	// +optional
	LastTransitionedTime *metav1.Time `json:"lastTransitionedTime,omitempty"`
	// +optionals
	LastCheck *metav1.Time `json:"lastCheck,omitempty"`
	// +optional
	Message *string `json:"message,omitempty"`
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`
	// Availibility over a rolling 1h period
	Uptime1H string `json:"uptime1h,omitempty"`
	// Average latency to complete all checks
	Latency1H string `json:"latency1h,omitempty"`
}

// +kubebuilder:object:root=true

// Canary is the Schema for the canaries API
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.interval`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Last Check",type=date,JSONPath=`.status.lastCheck`
// +kubebuilder:printcolumn:name="Uptime 1H",type=string,JSONPath=`.status.uptime1h`
// +kubebuilder:printcolumn:name="Latency 1H",type=string,JSONPath=`.status.latency1h`
// +kubebuilder:printcolumn:name="Last Transitioned",type=date,JSONPath=`.status.lastTransitionedTime`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Error",type=string,priority=1,JSONPath=`.status.errorMessage`
// +kubebuilder:subresource:status
type Canary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanarySpec   `json:"spec,omitempty"`
	Status CanaryStatus `json:"status,omitempty"`
}

func (c Canary) String() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

func (c Canary) GetAllLabels(extra map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range extra {
		labels["__"+k] = v
	}
	for k, v := range c.GetLabels() {
		labels[k] = v
	}
	if c.Spec.Severity != "" {
		labels[c.Spec.Severity] = "true"
	}
	if c.Spec.Owner != "" {
		labels[c.Spec.Owner] = "true"
	}
	return labels
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
