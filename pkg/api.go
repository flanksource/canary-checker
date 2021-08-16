package pkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/console"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Endpoint struct {
	String string
}

type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format("2006-01-02 15:04:05"))
	return []byte(stamp), nil
}

func (t *JSONTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		*t = JSONTime(time.Time{})
		return nil
	}
	x, err := time.Parse("2006-01-02 15:04:05", s)
	*t = JSONTime(x)
	return err
}

type CheckStatus struct {
	Status   bool     `json:"status"`
	Invalid  bool     `json:"invalid"`
	Time     JSONTime `json:"time"`
	Duration int      `json:"duration"`
	Message  string   `json:"message"`
	Error    string   `json:"error,omitempty"`
}

type Check struct {
	Key         string        `json:"key"`
	Type        string        `json:"type"`
	Name        string        `json:"name"`
	Namespace   string        `json:"namespace"`
  Labels       map[string]string `json:"labels"`
	RunnerLabels map[string]string `json:"runnerLabels"`
	CanaryName  string        `json:"canaryName"`
	Description string        `json:"description"`
	Endpoint    string        `json:"endpoint"`
	Uptime      string        `json:"uptime"`
	Latency     string        `json:"latency"`
	Statuses    []CheckStatus `json:"checkStatuses" mapstructure:"-"`
	Interval    uint64        `json:"interval"`
	Schedule    string        `json:"schedule"`
	Owner       string        `json:"owner"`
	Severity    string        `json:"severity"`
	IconURL     string        `json:"iconURL"`
	DisplayType string        `json:"displayType"`
	CheckCanary *v1.Canary    `json:"-"`
}

func (c Check) ID() string {
	return c.Key + c.Endpoint + c.Description
}

func (c Check) GetNamespace() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	return strings.Split(c.Name, "/")[0]
}

func (c Check) GetName() string {
	parts := strings.Split(c.Name, "/")
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[1]
}

type Checks []Check

func (c Checks) Len() int {
	return len(c)
}
func (c Checks) Less(i, j int) bool {
	return c[i].ToString() < c[j].ToString()
}

func (c Checks) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c Check) ToString() string {
	return fmt.Sprintf("%s-%s-%s", c.Name, c.Type, c.Description)
}

func (c Check) GetDescription() string {
	return c.Description
}

type Config struct {
	HTTP           []v1.HTTPCheck           `yaml:"http,omitempty" json:"http,omitempty"`
	DNS            []v1.DNSCheck            `yaml:"dns,omitempty" json:"dns,omitempty"`
	ContainerdPull []v1.ContainerdPullCheck `yaml:"containerdPull,omitempty" json:"containerdPull,omitempty"`
	ContainerdPush []v1.ContainerdPushCheck `yaml:"containerdPush,omitempty" json:"containerdPush,omitempty"`
	DockerPull     []v1.DockerPullCheck     `yaml:"docker,omitempty" json:"docker,omitempty"`
	DockerPush     []v1.DockerPushCheck     `yaml:"dockerPush,omitempty" json:"dockerPush,omitempty"`
	S3             []v1.S3Check             `yaml:"s3,omitempty" json:"s3,omitempty"`
	S3Bucket       []v1.S3BucketCheck       `yaml:"s3Bucket,omitempty" json:"s3Bucket,omitempty"`
	TCP            []v1.TCPCheck            `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Pod            []v1.PodCheck            `yaml:"pod,omitempty" json:"pod,omitempty"`
	LDAP           []v1.LDAPCheck           `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	ICMP           []v1.ICMPCheck           `yaml:"icmp,omitempty" json:"icmp,omitempty"`
	Postgres       []v1.PostgresCheck       `yaml:"postgres,omitempty" json:"postgres,omitempty"`
	Mssql          []v1.MssqlCheck          `yaml:"mssql,omitempty" json:"mssql,omitempty"`
	Redis          []v1.RedisCheck          `yaml:"redis,omitempty" json:"redis,omitempty"`
	Helm           []v1.HelmCheck           `yaml:"helm,omitempty" json:"helm,omitempty"`
	Namespace      []v1.NamespaceCheck      `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Interval       metav1.Duration          `yaml:"-" json:"interval,omitempty"`
}

type Checker interface {
	CheckArgs(args map[string]interface{}) *CheckResult
}

// URL information
type URL struct {
	IP       string
	Port     int
	Host     string
	Scheme   string
	Path     string
	Username string
	Password string
	Method   string
	Headers  map[string]string
	Body     string
}

type CheckResult struct {
	Pass        bool
	Invalid     bool
	Duration    int64
	Description string
	DisplayType string
	Message     string
	Error       string
	Metrics     []Metric
	// Check is the configuration
	Check external.Check
}

func (result CheckResult) GetDescription() string {
	if result.Check.GetDescription() != "" {
		return result.Check.GetDescription()
	}
	return result.Check.GetEndpoint()
}

func (result CheckResult) String() string {
	checkType := ""
	endpoint := ""
	if result.Check != nil {
		checkType = result.Check.GetType()
		endpoint = result.Check.GetEndpoint()
	}
	if result.Pass {
		return fmt.Sprintf("[%s] [%s] %s duration=%d %s %s", console.Greenf("PASS"), checkType, endpoint, result.Duration, result.Metrics, result.Message)
	}
	return fmt.Sprintf("[%s] [%s] %s duration=%d %s %s", console.Redf("FAIL"), checkType, endpoint, result.Duration, result.Metrics, result.Message)
}

type MetricType string

type Metric struct {
	Name   string
	Type   MetricType
	Labels map[string]string
	Value  float64
}

func (m Metric) String() string {
	return fmt.Sprintf("%s=%d", m.Name, int(m.Value))
}

func (e Endpoint) GetEndpoint() string {
	return e.String
}
