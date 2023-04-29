package pkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/canary-checker/pkg/labels"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"github.com/lib/pq"
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
	if err != nil {
		logger.Warnf("failed to parse time: %s", err)
		return nil
	}
	*t = JSONTime(x)
	return nil
}

type CheckStatus struct {
	Status   bool            `json:"status"`
	Invalid  bool            `json:"invalid,omitempty"`
	Time     string          `json:"time"`
	Duration int             `json:"duration"`
	Message  string          `json:"message,omitempty"`
	Error    string          `json:"error,omitempty"`
	Detail   interface{}     `json:"-"`
	Check    *external.Check `json:"check,omitempty"`
}

func (s CheckStatus) GetTime() (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", s.Time)
}

type Latency struct {
	Percentile99 float64 `json:"p99,omitempty" db:"p99"`
	Percentile97 float64 `json:"p97,omitempty" db:"p97"`
	Percentile95 float64 `json:"p95,omitempty" db:"p95"`
	Rolling1H    float64 `json:"rolling1h"`
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
	Passed   int        `json:"passed"`
	Failed   int        `json:"failed"`
	P100     float64    `json:"p100,omitempty"`
	LastPass *time.Time `json:"last_pass,omitempty"`
	LastFail *time.Time `json:"last_fail,omitempty"`
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
	Error    string `json:"error,omitempty"`
	Duration int    `json:"duration"`
	// Count is the number of times the check has been run in the specified time window
	Count int `json:"count,omitempty"`
}

type Canary struct {
	ID        uuid.UUID `gorm:"default:generate_ulid()"`
	Spec      types.JSON
	Labels    types.JSONStringMap
	Source    string
	Name      string
	Namespace string
	Checks    types.JSONStringMap `gorm:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `json:"deleted_at,omitempty" time_format:"postgres_timestamp"`
}

func (c Canary) GetCheckID(checkName string) string {
	return c.Checks[checkName]
}

func (c Canary) ToV1() (*v1.Canary, error) {
	canary := v1.Canary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Annotations: map[string]string{
				"source": c.Source,
			},
			Labels: c.Labels,
		},
	}
	var deletionTimestamp metav1.Time
	if c.DeletedAt != nil && !c.DeletedAt.IsZero() {
		deletionTimestamp = metav1.NewTime(*c.DeletedAt)
		canary.ObjectMeta.DeletionTimestamp = &deletionTimestamp
	}
	if err := json.Unmarshal(c.Spec, &canary.Spec); err != nil {
		logger.Debugf("Failed to unmarshal canary spec: %s", err)
		return nil, err
	}
	id := c.ID.String()
	canary.Status.PersistedID = &id
	canary.Status.Checks = c.Checks
	return &canary, nil
}

func CanaryFromV1(canary v1.Canary) (Canary, error) {
	spec, err := json.Marshal(canary.Spec)
	if err != nil {
		return Canary{}, err
	}
	var checks = make(map[string]string)
	if canary.Status.Checks != nil {
		checks = canary.Status.Checks
	}
	return Canary{
		Spec:      spec,
		Labels:    types.JSONStringMap(canary.Labels),
		Name:      canary.Name,
		Namespace: canary.Namespace,
		Source:    canary.Annotations["source"],
		Checks:    types.JSONStringMap(checks),
	}, nil
}

type Check struct {
	ID                 uuid.UUID           `json:"id" gorm:"default:generate_ulid()"`
	CanaryID           uuid.UUID           `json:"canary_id"`
	Spec               types.JSON          `json:"-"`
	Type               string              `json:"type"`
	Name               string              `json:"name"`
	CanaryName         string              `json:"canary_name" gorm:"-"`
	Namespace          string              `json:"namespace"  gorm:"-"`
	Labels             types.JSONStringMap `json:"labels" gorm:"type:jsonstringmap"`
	Description        string              `json:"description,omitempty"`
	Status             string              `json:"status,omitempty"`
	Uptime             Uptime              `json:"uptime"  gorm:"-"`
	Latency            Latency             `json:"latency"  gorm:"-"`
	Statuses           []CheckStatus       `json:"checkStatuses"  gorm:"-"`
	Owner              string              `json:"owner,omitempty"`
	Severity           string              `json:"severity,omitempty"`
	Icon               string              `json:"icon,omitempty"`
	DisplayType        string              `json:"displayType,omitempty"  gorm:"-"`
	Transformed        bool                `json:"transformed,omitempty"`
	LastRuntime        *time.Time          `json:"lastRuntime,omitempty"`
	LastTransitionTime *time.Time          `json:"lastTransitionTime,omitempty"`
	NextRuntime        *time.Time          `json:"nextRuntime,omitempty"`
	UpdatedAt          *time.Time          `json:"updatedAt,omitempty"`
	CreatedAt          *time.Time          `json:"createdAt,omitempty"`
	DeletedAt          *time.Time          `json:"deletedAt,omitempty"`
	SilencedAt         *time.Time          `json:"silencedAt,omitempty"`
	Canary             *v1.Canary          `json:"-" gorm:"-"`
}

func FromExternalCheck(canary Canary, check external.Check) Check {
	return Check{
		CanaryID:    canary.ID,
		Type:        check.GetType(),
		Icon:        check.GetIcon(),
		Description: check.GetDescription(),
		Name:        check.GetName(),
		Namespace:   canary.Namespace,
		CanaryName:  canary.Name,
		Labels:      labels.FilterLabels(canary.Labels),
	}
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
		Check:    &result.Check,
	}
}
func FromV1(canary v1.Canary, check external.Check, statuses ...CheckStatus) Check {
	canaryID, _ := uuid.Parse(canary.GetPersistedID())
	c := Check{
		Owner:    canary.Spec.Owner,
		Severity: canary.Spec.Severity,
		// DisplayType: check.DisplayType,
		Name:        check.GetName(),
		Description: check.GetDescription(),
		Icon:        check.GetIcon(),
		Namespace:   canary.Namespace,
		CanaryName:  canary.Name,
		CanaryID:    canaryID,
		Labels:      labels.FilterLabels(canary.GetAllLabels(check.GetLabels())),
		Statuses:    statuses,
		Type:        check.GetType(),
	}
	if _, exists := c.Labels["transformed"]; exists {
		c.Transformed = true
		delete(c.Labels, "transformed")
	}
	return c
}

func (c Check) GetID() string {
	return c.ID.String()
}

func (c Check) GetName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.Description
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

func (c Checks) Find(key string) *Check {
	for _, check := range c {
		if check.Name == key {
			return check
		}
	}
	return nil
}

func (c Check) ToString() string {
	return fmt.Sprintf("%s-%s-%s", c.Name, c.Type, c.Description)
}

func (c Check) GetDescription() string {
	return c.Description
}

type Checker interface {
	CheckArgs(args map[string]interface{}) *CheckResult
}

type Config struct {
	ID          *uuid.UUID          `json:"id,omitempty"`
	ConfigClass string              `json:"config_class,omitempty"`
	Name        string              `json:"name,omitempty"`
	Namespace   string              `json:"namespace,omitempty"`
	Spec        *types.JSONMap      `json:"spec,omitempty" gorm:"column:config"`
	Tags        types.JSONStringMap `json:"tags,omitempty"  gorm:"type:jsonstringmap"`
	ExternalID  pq.StringArray      `json:"external_id,omitempty" gorm:"type:text[]"`
	Type        string              `json:"type,omitempty"`
}

func (c Config) String() string {
	s := c.ConfigClass
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

func NewConfigs(configs []v1.Config) Configs {
	var pkgConfigs Configs
	for _, config := range configs {
		pkgConfigs = append(pkgConfigs, NewConfig(config))
	}
	return pkgConfigs
}

func NewConfig(config v1.Config) *Config {
	return &Config{
		Name:       config.Name,
		Namespace:  config.Namespace,
		Tags:       types.JSONStringMap(config.Tags),
		ExternalID: pq.StringArray(config.ID),
		Type:       config.Type,
	}
}

func ToV1Config(config Config) v1.Config {
	return v1.Config{
		Name:      config.Name,
		Namespace: config.Namespace,
		ID:        config.ExternalID,
		Type:      config.Type,
	}
}

func (c Config) GetSelectorID() string {
	selectorID, err := utils.GenerateJSONMD5Hash(ToV1Config(c))
	if err != nil {
		return ""
	}
	return selectorID
}

// ToJSONMap converts the struct to map[string]interface{} to
// be compatible with otto vm
func (c Config) ToJSONMap() map[string]interface{} {
	m := make(map[string]interface{})
	b, _ := json.Marshal(&c)
	_ = json.Unmarshal(b, &m)
	return m
}

type Configs []*Config

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

type SystemResult struct{}
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

type GenericCheck struct {
	v1.Description `yaml:",inline" json:",inline"`
	Type           string
	Endpoint       string
	Labels         map[string]string
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
	labels := t.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
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
				Labels:      labels,
			},
			Type:     t.Type,
			Endpoint: t.Endpoint,
		},
	}
}

func (t TransformedCheckResult) GetDescription() string {
	return t.Description
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
