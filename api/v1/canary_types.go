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
	"time"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/commons/logger"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ResultMode string

const (
	JunitResultMode = "junit"
)

// CanarySpec defines the desired state of Canary
type CanarySpec struct {
	//+kubebuilder:default=1
	//+optional
	// Replicas pauses the canary if = 0.
	Replicas *int `yaml:"replicas,omitempty" json:"replicas,omitempty"`

	Env                map[string]VarSource      `yaml:"env,omitempty" json:"env,omitempty"`
	HTTP               []HTTPCheck               `yaml:"http,omitempty" json:"http,omitempty"`
	DNS                []DNSCheck                `yaml:"dns,omitempty" json:"dns,omitempty"`
	DockerPull         []DockerPullCheck         `yaml:"docker,omitempty" json:"docker,omitempty"`
	DockerPush         []DockerPushCheck         `yaml:"dockerPush,omitempty" json:"dockerPush,omitempty"`
	ContainerdPull     []ContainerdPullCheck     `yaml:"containerd,omitempty" json:"containerd,omitempty"`
	ContainerdPush     []ContainerdPushCheck     `yaml:"containerdPush,omitempty" json:"containerdPush,omitempty"`
	S3                 []S3Check                 `yaml:"s3,omitempty" json:"s3,omitempty"`
	TCP                []TCPCheck                `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Pod                []PodCheck                `yaml:"pod,omitempty" json:"pod,omitempty"`
	LDAP               []LDAPCheck               `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	ICMP               []ICMPCheck               `yaml:"icmp,omitempty" json:"icmp,omitempty"`
	Postgres           []PostgresCheck           `yaml:"postgres,omitempty" json:"postgres,omitempty"`
	Mssql              []MssqlCheck              `yaml:"mssql,omitempty" json:"mssql,omitempty"`
	Mysql              []MysqlCheck              `yaml:"mysql,omitempty" json:"mysql,omitempty"`
	Restic             []ResticCheck             `yaml:"restic,omitempty" json:"restic,omitempty"`
	Jmeter             []JmeterCheck             `yaml:"jmeter,omitempty" json:"jmeter,omitempty"`
	Junit              []JunitCheck              `yaml:"junit,omitempty" json:"junit,omitempty"`
	Helm               []HelmCheck               `yaml:"helm,omitempty" json:"helm,omitempty"`
	Namespace          []NamespaceCheck          `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Redis              []RedisCheck              `yaml:"redis,omitempty" json:"redis,omitempty"`
	Prometheus         []PrometheusCheck         `yaml:"prometheus,omitempty" json:"prometheus,omitempty"`
	MongoDB            []MongoDBCheck            `yaml:"mongodb,omitempty" json:"mongodb,omitempty"`
	CloudWatch         []CloudWatchCheck         `yaml:"cloudwatch,omitempty" json:"cloudwatch,omitempty"`
	PubSub             []PubSubCheck             `yaml:"pubsub,omitempty" json:"pubsub,omitempty"`
	GitHub             []GitHubCheck             `yaml:"github,omitempty" json:"github,omitempty"`
	GitProtocol        []GitProtocolCheck        `yaml:"gitProtocol,omitempty" json:"gitProtocol,omitempty"`
	Kubernetes         []KubernetesCheck         `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	KubernetesResource []KubernetesResourceCheck `yaml:"kubernetesResource,omitempty" json:"kubernetesResource,omitempty"`
	Folder             []FolderCheck             `yaml:"folder,omitempty" json:"folder,omitempty"`
	Exec               []ExecCheck               `yaml:"exec,omitempty" json:"exec,omitempty"`
	AwsConfig          []AwsConfigCheck          `yaml:"awsConfig,omitempty" json:"awsConfig,omitempty"`
	AwsConfigRule      []AwsConfigRuleCheck      `yaml:"awsConfigRule,omitempty" json:"awsConfigRule,omitempty"`
	DatabaseBackup     []DatabaseBackupCheck     `yaml:"databaseBackup,omitempty" json:"databaseBackup,omitempty"`
	Catalog            []CatalogCheck            `yaml:"catalog,omitempty" json:"catalog,omitempty"`
	Opensearch         []OpenSearchCheck         `yaml:"opensearch,omitempty" json:"opensearch,omitempty"`
	Elasticsearch      []ElasticsearchCheck      `yaml:"elasticsearch,omitempty" json:"elasticsearch,omitempty"`
	AlertManager       []AlertManagerCheck       `yaml:"alertmanager,omitempty" json:"alertmanager,omitempty"`
	Dynatrace          []DynatraceCheck          `yaml:"dynatrace,omitempty" json:"dynatrace,omitempty"`
	AzureDevops        []AzureDevopsCheck        `yaml:"azureDevops,omitempty" json:"azureDevops,omitempty"`
	Webhook            *WebhookCheck             `yaml:"webhook,omitempty" json:"webhook,omitempty"`
	// interval (in seconds) to run checks on Deprecated in favor of Schedule
	Interval uint64 `yaml:"interval,omitempty" json:"interval,omitempty"`
	// Schedule to run checks on. Supports all cron expression, example: '30 3-6,20-23 * * *'. For more info about cron expression syntax see https://en.wikipedia.org/wiki/Cron
	// Also supports golang duration, can be set as '@every 1m30s' which runs the check every 1 minute and 30 seconds.
	// If both schedule and interval are empty, the canary will not run
	Schedule   string     `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Icon       string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Severity   string     `yaml:"severity,omitempty" json:"severity,omitempty"`
	Owner      string     `yaml:"owner,omitempty" json:"owner,omitempty"`
	ResultMode ResultMode `yaml:"resultMode,omitempty" json:"resultMode,omitempty"`
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
	for _, check := range spec.Mysql {
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
	for _, check := range spec.Prometheus {
		checks = append(checks, check)
	}
	for _, check := range spec.MongoDB {
		checks = append(checks, check)
	}
	for _, check := range spec.CloudWatch {
		checks = append(checks, check)
	}
	for _, check := range spec.PubSub {
		checks = append(checks, check)
	}
	for _, check := range spec.GitHub {
		checks = append(checks, check)
	}
	for _, check := range spec.GitProtocol {
		checks = append(checks, check)
	}
	for _, check := range spec.Kubernetes {
		checks = append(checks, check)
	}
	for _, check := range spec.KubernetesResource {
		checks = append(checks, check)
	}
	for _, check := range spec.Folder {
		checks = append(checks, check)
	}
	for _, check := range spec.Exec {
		checks = append(checks, check)
	}
	for _, check := range spec.AwsConfig {
		checks = append(checks, check)
	}
	for _, check := range spec.AwsConfigRule {
		checks = append(checks, check)
	}
	for _, check := range spec.DatabaseBackup {
		checks = append(checks, check)
	}
	for _, check := range spec.Catalog {
		checks = append(checks, check)
	}
	for _, check := range spec.Elasticsearch {
		checks = append(checks, check)
	}
	for _, check := range spec.AlertManager {
		checks = append(checks, check)
	}
	for _, check := range spec.AzureDevops {
		checks = append(checks, check)
	}
	for _, check := range spec.Dynatrace {
		checks = append(checks, check)
	}
	for _, check := range spec.Opensearch {
		checks = append(checks, check)
	}
	return checks
}

// KeepOnly removes all the checks from the spec that do not
// match the given name (exactly)
func (spec CanarySpec) KeepOnly(names ...string) CanarySpec {
	spec.HTTP = lo.Filter(spec.HTTP, func(c HTTPCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.DNS = lo.Filter(spec.DNS, func(c DNSCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.DockerPull = lo.Filter(spec.DockerPull, func(c DockerPullCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.DockerPush = lo.Filter(spec.DockerPush, func(c DockerPushCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.ContainerdPull = lo.Filter(spec.ContainerdPull, func(c ContainerdPullCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.ContainerdPush = lo.Filter(spec.ContainerdPush, func(c ContainerdPushCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.S3 = lo.Filter(spec.S3, func(c S3Check, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.TCP = lo.Filter(spec.TCP, func(c TCPCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Pod = lo.Filter(spec.Pod, func(c PodCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.LDAP = lo.Filter(spec.LDAP, func(c LDAPCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.ICMP = lo.Filter(spec.ICMP, func(c ICMPCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Postgres = lo.Filter(spec.Postgres, func(c PostgresCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Mssql = lo.Filter(spec.Mssql, func(c MssqlCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Mysql = lo.Filter(spec.Mysql, func(c MysqlCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Restic = lo.Filter(spec.Restic, func(c ResticCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Jmeter = lo.Filter(spec.Jmeter, func(c JmeterCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Junit = lo.Filter(spec.Junit, func(c JunitCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Helm = lo.Filter(spec.Helm, func(c HelmCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Namespace = lo.Filter(spec.Namespace, func(c NamespaceCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Redis = lo.Filter(spec.Redis, func(c RedisCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Prometheus = lo.Filter(spec.Prometheus, func(c PrometheusCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.MongoDB = lo.Filter(spec.MongoDB, func(c MongoDBCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.CloudWatch = lo.Filter(spec.CloudWatch, func(c CloudWatchCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.PubSub = lo.Filter(spec.PubSub, func(c PubSubCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.GitHub = lo.Filter(spec.GitHub, func(c GitHubCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.GitProtocol = lo.Filter(spec.GitProtocol, func(c GitProtocolCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Kubernetes = lo.Filter(spec.Kubernetes, func(c KubernetesCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Folder = lo.Filter(spec.Folder, func(c FolderCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Exec = lo.Filter(spec.Exec, func(c ExecCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.AwsConfig = lo.Filter(spec.AwsConfig, func(c AwsConfigCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.AwsConfigRule = lo.Filter(spec.AwsConfigRule, func(c AwsConfigRuleCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.DatabaseBackup = lo.Filter(spec.DatabaseBackup, func(c DatabaseBackupCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Catalog = lo.Filter(spec.Catalog, func(c CatalogCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Opensearch = lo.Filter(spec.Opensearch, func(c OpenSearchCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Elasticsearch = lo.Filter(spec.Elasticsearch, func(c ElasticsearchCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.AlertManager = lo.Filter(spec.AlertManager, func(c AlertManagerCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.Dynatrace = lo.Filter(spec.Dynatrace, func(c DynatraceCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})
	spec.AzureDevops = lo.Filter(spec.AzureDevops, func(c AzureDevopsCheck, _ int) bool {
		return lo.Contains(names, c.GetName())
	})

	return spec
}

const NoSchedule = "NoSchedule"

func (spec CanarySpec) GetSchedule() string {
	if spec.Schedule != "" {
		return spec.Schedule
	}
	if spec.Interval > 0 {
		return fmt.Sprintf("@every %ds", spec.Interval)
	}
	return NoSchedule
}

func (c Canary) IsTrace() bool {
	return c.Annotations != nil && c.Annotations["trace"] == "true" //nolint
}

func (c Canary) IsDebug() bool {
	return c.Annotations != nil && c.Annotations["debug"] == "true"
}

func (c Canary) GetKey(check external.Check) string {
	data, err := json.Marshal(check)
	if err != nil {
		logger.Debugf("error marshalling the check")
	}
	hash := md5.Sum(data)
	return fmt.Sprintf("%s/%s:%s/%s:%s", c.ID(), check.GetType(), check.GetName(), c.GetDescription(check), hex.EncodeToString(hash[:]))
}

func (c Canary) GetDescription(check external.Check) string {
	if check.GetDescription() != "" {
		return check.GetDescription()
	}
	return check.GetEndpoint()
}

func (c Canary) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: c.Name, Namespace: c.Namespace}
}

func (c *Canary) SetRunnerName(name string) {
	c.Status.runnerName = name
}

func (c *Canary) GetRunnerName() string {
	return c.Status.runnerName
}

type CanaryStatusCondition string

var (
	Passed  CanaryStatusCondition = "Passed"
	Failed  CanaryStatusCondition = "Failed"
	Invalid CanaryStatusCondition = "Invalid"
)

// CanaryStatus defines the observed state of Canary
type CanaryStatus struct {
	PersistedID *string `json:"persistedID,omitempty"`
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
	// contains the name and id of the checks associated with the canary
	Checks map[string]string `json:"checks,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
	// +optional
	ChecksStatus map[string]*CheckStatus `json:"checkStatus,omitempty"`
	// Availibility over a rolling 1h period
	Uptime1H string `json:"uptime1h,omitempty"`
	// Average latency to complete all checks
	Latency1H string `json:"latency1h,omitempty"`
	// used for keeping history of the checks
	runnerName string `json:"-"`
	// Replicas keep track of the number of replicas
	Replicas int    `json:"replicas,omitempty"`
	Selector string `json:"selector,omitempty"` // for autoscaling
}

func (c Canary) GetCheckID(checkName string) string {
	return c.Status.Checks[checkName]
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
// +kubebuilder:printcolumn:name="Replicas",type=integer,priority=1,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.interval`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Last Check",type=date,JSONPath=`.status.lastCheck`
// +kubebuilder:printcolumn:name="Uptime 1H",type=string,JSONPath=`.status.uptime1h`
// +kubebuilder:printcolumn:name="Latency 1H",type=string,JSONPath=`.status.latency1h`
// +kubebuilder:printcolumn:name="Last Transitioned",type=date,JSONPath=`.status.lastTransitionedTime`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Error",type=string,priority=1,JSONPath=`.status.errorMessage`
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
type Canary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanarySpec   `json:"spec,omitempty"`
	Status CanaryStatus `json:"status,omitempty"`
}

func NewCanaryFromSpec(name, namespace string, spec CanarySpec) Canary {
	return Canary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

func (c Canary) String() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

func (c Canary) GetAllLabels(extra map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range c.GetLabels() {
		labels[k] = v
	}
	for k, v := range extra {
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
	return fmt.Sprintf("%s/%s/%s", c.GetRunnerName(), c.Namespace, c.Name)
}

func (c Canary) GetPersistedID() string {
	return string(c.GetUID())
}

func (c Canary) NextRuntime(lastRuntime time.Time) (*time.Time, error) {
	if c.Spec.Schedule != "" {
		schedule, err := cron.ParseStandard(c.Spec.Schedule)
		if err != nil {
			return nil, err
		}
		t := schedule.Next(time.Now())
		return &t, nil
	}
	t := lastRuntime.Add(time.Duration(c.Spec.Interval) * time.Second)
	return &t, nil
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
