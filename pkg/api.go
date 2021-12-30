package pkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/labels"
	"github.com/flanksource/canary-checker/pkg/utils"
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
	Status   bool        `json:"status"`
	Invalid  bool        `json:"invalid,omitempty"`
	Time     string      `json:"time"`
	Duration int         `json:"duration"`
	Message  string      `json:"message,omitempty"`
	Error    string      `json:"error,omitempty"`
	Detail   interface{} `json:"-"`
}

func (s CheckStatus) GetTime() (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", s.Time)
}

type Latency struct {
	Percentile99 float64 `json:"p99,omitempty" db:"p99"`
	Percentile97 float64 `json:"p97,omitempty" db:"p97"`
	Percentile95 float64 `json:"p95,omitempty" db:"p95"`
	Rolling1H    float64 `json:"rolling1h" db`
}

func (l Latency) String() string {
	s := ""
	if l.Percentile99 != 0 {
		s += fmt.Sprintf("p99=%s", utils.Age(time.Duration(l.Percentile99)*time.Millisecond))
	}
	if l.Percentile95 != 0 {
		s += fmt.Sprintf("p95=%s", utils.Age(time.Duration(l.Percentile95)*time.Millisecond))
	}
	if l.Percentile97 != 0 {
		s += fmt.Sprintf("p97=%s", utils.Age(time.Duration(l.Percentile97)*time.Millisecond))
	}
	if l.Rolling1H != 0 {
		s += fmt.Sprintf("rolling1h=%s", utils.Age(time.Duration(l.Rolling1H)*time.Millisecond))
	}
	return s
}

type Uptime struct {
	Passed int     `json:"passed"`
	Failed int     `json:"failed"`
	P100   float64 `json:"p100,omitempty"`
}

func (u Uptime) String() string {
	if u.Passed == 0 && u.Failed == 0 {
		return ""
	}
	if u.Passed == 0 {
		return fmt.Sprintf("0/%d 0%%", u.Failed)
	}
	percentage := 100.0 * (1 - (float64(u.Failed) / float64(u.Passed+u.Failed)))
	return fmt.Sprintf("%d/%d (%0.1f%%)", u.Passed, u.Passed+u.Failed, percentage)
}

type Timeseries struct {
	Key      string `json:"key,omitempty"`
	Time     string `json:"time,omitempty"`
	Status   bool   `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
	Duration int    `json:"duration,omitempty"`
}

type Check struct {
	Key          string            `json:"key"`
	Type         string            `json:"type"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	RunnerLabels map[string]string `json:"runnerLabels,omitempty"`
	CanaryName   string            `json:"canaryName"`
	Description  string            `json:"description,omitempty"`
	Endpoint     string            `json:"endpoint,omitempty"`
	Uptime       Uptime            `json:"uptime" db:""`
	Latency      Latency           `json:"latency" db:""`
	Statuses     []CheckStatus     `json:"checkStatuses"`
	Interval     uint64            `json:"interval,omitempty"`
	Schedule     string            `json:"schedule,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Severity     string            `json:"severity,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	DisplayType  string            `json:"displayType,omitempty"`
	RunnerName   string            `json:"runnerName,omitempty"`
	// Specify the canary id, <runner>/<namespace>/<name>
	ID     string     `json:"id"`
	Canary *v1.Canary `json:"-"`
}

func (c Check) String() string {
	s := ""

	if c.Name != "" {
		s += "name=" + c.Name + " "
	}
	if c.Key != "" {
		s += "key=" + c.Key + " "
	}
	if c.Type != "" {
		s += "type=" + c.Type + " "
	}
	if c.Namespace != "" {
		s += "namespace=" + c.Namespace + " "
	}
	if c.CanaryName != "" {
		s += "canary=" + c.CanaryName + " "
	}
	if c.Description != "" {
		s += "description=" + c.Description + " "
	}
	if c.Endpoint != "" {
		s += "endpoint=" + c.Endpoint + " "
	}
	if c.Uptime.String() != "" {
		s += "uptime=" + c.Uptime.String() + " "
	}
	if c.Latency.String() != "" {
		s += "latency=" + c.Latency.String() + " "
	}
	if c.Interval != 0 {
		s += "interval=" + fmt.Sprintf("%d", c.Interval) + " "
	}
	if c.Schedule != "" {
		s += "schedule=" + c.Schedule + " "
	}
	if c.Owner != "" {
		s += "owner=" + c.Owner + " "
	}
	if c.Severity != "" {
		s += "severity=" + c.Severity + " "
	}
	if c.Icon != "" {
		s += "icon=" + c.Icon + " "
	}
	if c.DisplayType != "" {
		s += "displayType=" + c.DisplayType + " "
	}
	if c.RunnerName != "" {
		s += "runner=" + c.RunnerName + " "
	}
	if c.ID != "" {
		s += "id=" + c.ID + " "
	}
	s += "statuses=" + fmt.Sprintf("%d", len(c.Statuses))
	return s
}

func FromResult(result CheckResult) CheckStatus {
	return CheckStatus{
		Status:   result.Pass,
		Invalid:  result.Invalid,
		Duration: int(result.Duration),
		Time:     time.Now().UTC().Format(time.RFC3339),
		Message:  result.Message,
		Error:    result.Error,
		Detail:   result.Detail,
	}
}
func FromV1(canary v1.Canary, check external.Check, statuses ...CheckStatus) Check {
	return Check{
		Canary:      &canary,
		CanaryName:  canary.Name,
		Description: check.GetDescription(),
		Endpoint:    check.GetEndpoint(),
		Icon:        check.GetIcon(),
		ID:          canary.ID(),
		Interval:    canary.Spec.Interval,
		Key:         canary.GetKey(check),
		Labels:      labels.FilterLabels(canary.GetAllLabels(nil)),
		Name:        check.GetName(),
		Namespace:   canary.Namespace,
		Owner:       canary.Spec.Owner,
		RunnerName:  canary.GetRunnerName(),
		Schedule:    canary.Spec.Schedule,
		Severity:    canary.Spec.Severity,
		Statuses:    statuses,
		Type:        check.GetType(),
	}
}

func (c Check) GetID() string {
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

type Checks []*Check

func (c Checks) Len() int {
	return len(c)
}
func (c Checks) Less(i, j int) bool {
	return c[i].ToString() < c[j].ToString()
}

func (c Checks) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c Checks) String() string {
	var s string
	for _, check := range c {
		s += check.String() + "\n"
	}
	return s
}

func (c Checks) Find(key string) *Check {
	for _, check := range c {
		if check.Key == key {
			return check
		}
	}
	return nil
}

func (c Checks) Merge(from Checks) Checks {
	for _, check := range from {
		match := c.Find(check.Key)
		if match == nil {
			c = append(c, check)
		} else {
			match.Statuses = append(match.Statuses, check.Statuses...)
		}
	}
	return c
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
	Start       time.Time
	Pass        bool
	Invalid     bool
	Detail      interface{}
	Data        map[string]interface{}
	Duration    int64
	Description string
	DisplayType string
	Message     string
	Error       string
	Metrics     []Metric
	// Check is the configuration
	Check  external.Check
	Canary v1.Canary
}

type GenericCheck struct {
	v1.Description `yaml:",inline" json:",inline"`
	Type           string
	Endpoint       string
}

func (generic GenericCheck) GetType() string {
	return generic.Type
}

func (generic GenericCheck) GetEndpoint() string {
	return generic.Endpoint
}

type TransformedCheckResult struct {
	Start       time.Time              `json:"start,omitempty"`
	Pass        bool                   `json:"pass,omitempty"`
	Invalid     bool                   `json:"invalid,omitempty"`
	Detail      interface{}            `json:"detail,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Duration    int64                  `json:"duration,omitempty"`
	Description string                 `json:"description,omitempty"`
	DisplayType string                 `json:"displayType,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Namespace   string                 `json:"namespace,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Endpoint    string                 `json:"endpoint,omitempty"`
}

func (t TransformedCheckResult) ToCheckResult() CheckResult {
	return CheckResult{
		Start:       t.Start,
		Pass:        t.Pass,
		Invalid:     t.Invalid,
		Detail:      t.Detail,
		Data:        t.Data,
		Duration:    t.Duration,
		Description: t.Description,
		DisplayType: t.DisplayType,
		Message:     t.Message,
		Error:       t.Error,
		Check: GenericCheck{
			Description: v1.Description{
				Description: t.Description,
				Name:        t.Name,
				Icon:        t.Icon,
			},
			Type:     t.Type,
			Endpoint: t.Endpoint,
		},
	}
}

func (result TransformedCheckResult) GetDescription() string {
	return result.Description
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
		endpoint = result.Check.GetName()
		if endpoint == "" {
			endpoint = result.Check.GetDescription()
		}
		if endpoint == "" {
			endpoint = result.Check.GetEndpoint()
		}
		endpoint = result.Canary.Namespace + "/" + result.Canary.Name + "/" + endpoint
	}

	if result.Pass {
		return fmt.Sprintf("%s [%s] %s duration=%d %s", console.Greenf("PASS"), checkType, endpoint, result.Duration, result.Message)
	}
	return fmt.Sprintf("%s [%s] %s duration=%d %s %s", console.Redf("FAIL"), checkType, endpoint, result.Duration, result.Message, result.Error)
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
