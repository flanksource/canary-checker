package pkg

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/labels"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
	cUtils "github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
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
	Status     bool            `json:"status"`
	Invalid    bool            `json:"invalid,omitempty"`
	Time       string          `json:"time"`
	DurationMs int32           `json:"duration"`
	Message    string          `json:"message,omitempty"`
	Error      string          `json:"error,omitempty"`
	Detail     interface{}     `json:"-"`
	Check      *external.Check `json:"check,omitempty"`
}

func (s CheckStatus) GetTime() (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", s.Time)
}

type Canary struct {
	ID          uuid.UUID `gorm:"default:generate_ulid()"`
	AgentID     uuid.UUID
	Spec        types.JSON          `json:"spec"`
	Labels      types.JSONStringMap `json:"labels"`
	Source      string
	Name        string
	Namespace   string
	Checks      types.JSONStringMap `gorm:"-"`
	Annotations types.JSONStringMap `json:"annotations,omitempty"`
	CreatedAt   time.Time
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" time_format:"postgres_timestamp"`
}

func (c Canary) FindChecks(db *gorm.DB) (checks Checks, err error) {
	err = db.Table("checks").Where("canary_id = ?", c.ID).Find(&checks).Error
	return
}

func (c Canary) GetCheckID(checkName string) string {
	return c.Checks[checkName]
}

func (c Canary) ToV1() (*v1.Canary, error) {
	annotations := c.Annotations
	if annotations == nil {
		annotations = make(types.JSONStringMap)
	}
	annotations["source"] = c.Source
	canary := v1.Canary{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.Name,
			Namespace:   c.Namespace,
			Annotations: annotations,
			Labels:      c.Labels,
			UID:         k8stypes.UID(c.ID.String()),
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

	canary.Status.Checks = c.Checks
	canary.ObjectMeta.Annotations = collections.MergeMap(canary.ObjectMeta.Annotations, c.Annotations)

	return &canary, nil
}

func (c Canary) GetSpec() (v1.CanarySpec, error) {
	var spec v1.CanarySpec
	err := json.Unmarshal(c.Spec, &spec)
	return spec, err
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
		Spec:        spec,
		Labels:      types.JSONStringMap(canary.Labels),
		Annotations: types.JSONStringMap(canary.Annotations),
		Name:        canary.Name,
		Namespace:   canary.Namespace,
		Source:      canary.Annotations["source"],
		Checks:      types.JSONStringMap(checks),
	}, nil
}

type Check struct {
	ID                 uuid.UUID                `json:"id" gorm:"default:generate_ulid()"`
	CanaryID           uuid.UUID                `json:"canary_id"`
	Spec               types.JSON               `json:"-"`
	Type               string                   `json:"type"`
	Name               string                   `json:"name"`
	CanaryName         string                   `json:"canary_name" gorm:"-"`
	Namespace          string                   `json:"namespace"`
	Labels             types.JSONStringMap      `json:"labels" gorm:"type:jsonstringmap"`
	Description        string                   `json:"description,omitempty"`
	Status             models.CheckHealthStatus `json:"status,omitempty"`
	Uptime             types.Uptime             `json:"uptime"  gorm:"-"`
	Latency            types.Latency            `json:"latency"  gorm:"-"`
	Statuses           []CheckStatus            `json:"checkStatuses"  gorm:"-"`
	Owner              string                   `json:"owner,omitempty"`
	Severity           string                   `json:"severity,omitempty"`
	Icon               string                   `json:"icon,omitempty"`
	DisplayType        string                   `json:"displayType,omitempty"  gorm:"-"`
	Transformed        bool                     `json:"transformed,omitempty"`
	LastRuntime        *time.Time               `json:"lastRuntime,omitempty"`
	LastTransitionTime *time.Time               `json:"lastTransitionTime,omitempty"`
	NextRuntime        *time.Time               `json:"nextRuntime,omitempty"`
	UpdatedAt          *time.Time               `json:"updatedAt,omitempty"`
	CreatedAt          *time.Time               `json:"createdAt,omitempty"`
	DeletedAt          *time.Time               `json:"deletedAt,omitempty"`
	SilencedAt         *time.Time               `json:"silencedAt,omitempty"`
	Canary             *v1.Canary               `json:"-" gorm:"-"`

	// These are calculated for the selected date range
	EarliestRuntime *time.Time `json:"earliestRuntime,omitempty" gorm:"-"`
	LatestRuntime   *time.Time `json:"latestRuntime,omitempty" gorm:"-"`
	TotalRuns       int        `json:"totalRuns,omitempty" gorm:"-"`
}

func FromExternalCheck(canary Canary, check external.Check) Check {
	return Check{
		CanaryID:    canary.ID,
		Type:        check.GetType(),
		Icon:        check.GetIcon(),
		Description: check.GetDescription(),
		Name:        check.GetName(),
		Namespace:   cUtils.Coalesce(check.GetNamespace(), canary.Namespace),
		CanaryName:  canary.Name,
		Labels:      labels.FilterLabels(canary.Labels),
	}
}

func ellipsis(str string, length int) string {
	if length <= 3 || len(str) <= length {
		return str
	}
	str = strings.TrimSpace(str)

	return strings.TrimSpace(str[0:length-3]) + "..."
}

func TruncateMessage(s string) string {
	return ellipsis(s, properties.Int(4*1024, "canary.status.max.message"))
}

func TruncateError(s string) string {
	return ellipsis(s, properties.Int(128*1024, "canary.status.max.error"))
}

func CheckStatusFromResult(result CheckResult) CheckStatus {
	cs := CheckStatus{
		Status:  result.Pass,
		Invalid: result.Invalid,
		Time:    time.Now().UTC().Format(time.RFC3339),
		Message: TruncateMessage(result.Message),
		Error:   TruncateError(result.Error),
		Detail:  result.Detail,
		Check:   &result.Check,
	}

	// For check duration over ~25 days, we limit it to MaxInt32 milliseconds.
	if result.Duration > math.MaxInt32 && false {
		cs.DurationMs = math.MaxInt32
	} else {
		cs.DurationMs = int32(result.Duration)
	}

	return cs
}

func FromV1(canary v1.Canary, check external.Check, statuses ...CheckStatus) Check {
	canaryID, _ := uuid.Parse(canary.GetPersistedID())
	checkID, _ := uuid.Parse(canary.GetCheckID(check.GetName()))

	if customID := check.GetCustomUUID(); customID != uuid.Nil {
		checkID = customID
	}

	c := Check{
		ID:       checkID,
		Owner:    canary.Spec.Owner,
		Severity: canary.Spec.Severity,
		// DisplayType: check.DisplayType,
		Name:        check.GetName(),
		Namespace:   cUtils.Coalesce(check.GetNamespace(), canary.Namespace),
		Description: check.GetDescription(),
		Icon:        check.GetIcon(),
		CanaryName:  canary.Name,
		CanaryID:    canaryID,
		Labels:      labels.FilterLabels(canary.GetAllLabels(check.GetLabels())),
		Statuses:    statuses,
		Type:        check.GetType(),
		Canary:      &canary,
	}

	if _, exists := c.Labels["transformed"]; exists {
		c.Transformed = true
		delete(c.Labels, "transformed")
	}

	if canary.DeletionTimestamp != nil && !canary.DeletionTimestamp.Time.IsZero() {
		c.DeletedAt = &canary.DeletionTimestamp.Time
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

func (c Check) GetNamespace() string {
	return c.Namespace
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

func (c Check) String() string {
	return fmt.Sprintf("%s/%s type=%s", c.Namespace, c.Name, c.Type)
}

func (c Check) GetDescription() string {
	return c.Description
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

type SystemResult struct{}

type ArtifactResult struct {
	ContentType string
	Path        string
	Content     io.ReadCloser
	Connection  string
}

type CheckResult struct {
	Name        string                 `json:"name,omitempty"`
	Start       time.Time              `json:"start,omitempty"`
	Pass        bool                   `json:"pass,omitempty"`
	Invalid     bool                   `json:"invalid,omitempty"`
	Detail      interface{}            `json:"detail,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Duration    int64                  `json:"duration,omitempty"`
	Description string                 `json:"description,omitempty"`
	DisplayType string                 `json:"display_type,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Namespace   string                 `json:"namespace,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metrics     []Metric               `json:"metrics,omitempty"`
	Transformed bool                   `json:"transformed,omitempty"`
	// Artifacts is the generated artifacts
	Artifacts []artifacts.Artifact `json:"artifacts,omitempty"`
	// Check is the configuration
	Check  external.Check `json:"-"`
	Canary v1.Canary      `json:"-"`
	// ParentCheck is the parent check of a transformed check
	ParentCheck external.Check `json:"-"`
	ErrorObject error          `json:"-"`

	InternalError bool `json:"-"`
}

func (result CheckResult) LoggerName() string {
	if result.Name != "" {
		return result.Canary.Name + "." + result.Name
	} else if result.Check.GetName() != "" {
		return result.Canary.Name + "." + result.Check.GetName()
	}
	return result.Canary.Name
}

func (result CheckResult) GetContext() map[string]any {
	c := map[string]any{
		"name":      result.Name,
		"data":      result.Data,
		"details":   result.Detail,
		"labels":    result.Labels,
		"namespace": result.Namespace,
		"metrics":   result.Metrics,
	}
	return c
}

func (result CheckResult) GetDescription() string {
	if result.Check.GetDescription() != "" {
		return result.Check.GetDescription()
	}
	return result.Check.GetEndpoint()
}

func (result CheckResult) GetName() string {
	if result.Check == nil {
		return ""
	}
	endpoint := result.Check.GetName()
	if endpoint == "" {
		endpoint = result.Check.GetDescription()
	}
	if endpoint == "" {
		endpoint = result.Check.GetEndpoint()
	}
	return endpoint
}

func (result CheckResult) String() string {
	if result.Pass {
		return fmt.Sprintf("%s duration=%d %s", console.Greenf("PASS"), result.Duration, result.Message)
	} else if result.ErrorObject != nil {
		return fmt.Sprintf("%s duration=%d %s %+v", console.Redf("FAIL"), result.Duration, result.Message, result.ErrorObject)
	}
	return fmt.Sprintf("%s duration=%d %s %s", console.Redf("FAIL"), result.Duration, result.Message, result.Error)
}

type GenericCheck struct {
	v1.Description `yaml:",inline" json:",inline"`
	Type           string
	Endpoint       string
	CustomID       uuid.UUID
}

func (generic GenericCheck) GetType() string {
	return generic.Type
}

func (generic GenericCheck) GetCustomUUID() uuid.UUID {
	return generic.CustomID
}

func (generic GenericCheck) ShouldMarkFailOnEmpty() bool {
	return generic.MarkFailOnEmpty
}

func (generic GenericCheck) GetHash() string {
	h, _ := utils.GenerateJSONMD5Hash(generic)
	return h
}

func (generic GenericCheck) GetEndpoint() string {
	return generic.Endpoint
}

type TransformedCheckResult struct {
	ID                      uuid.UUID              `json:"id,omitempty"`
	Start                   *time.Time             `json:"start,omitempty"`
	Pass                    *bool                  `json:"pass,omitempty"`
	Invalid                 *bool                  `json:"invalid,omitempty"`
	Detail                  interface{}            `json:"detail,omitempty"`
	Data                    map[string]interface{} `json:"data,omitempty"`
	DeletedAt               *time.Time             `json:"deletedAt,omitempty"`
	Duration                *int64                 `json:"duration,omitempty"`
	Description             string                 `json:"description,omitempty"`
	DisplayType             string                 `json:"displayType,omitempty"`
	Message                 string                 `json:"message,omitempty"`
	Error                   string                 `json:"error,omitempty"`
	Name                    string                 `json:"name,omitempty"`
	Labels                  map[string]string      `json:"labels,omitempty"`
	Namespace               string                 `json:"namespace,omitempty"`
	Metrics                 []Metric               `json:"metrics,omitempty"`
	Icon                    string                 `json:"icon,omitempty"`
	Type                    string                 `json:"type,omitempty"`
	Endpoint                string                 `json:"endpoint,omitempty"`
	TransformDeleteStrategy string                 `json:"transformDeleteStrategy,omitempty"`
}

func (t TransformedCheckResult) ToCheckResult() CheckResult {
	labels := t.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	return CheckResult{
		Start:       utils.Deref(t.Start, time.Now()),
		Pass:        utils.Deref(t.Pass, false),
		Invalid:     utils.Deref(t.Invalid, false),
		Detail:      t.Detail,
		Data:        t.Data,
		Duration:    utils.Deref(t.Duration, 0),
		Description: t.Description,
		DisplayType: t.DisplayType,
		Message:     t.Message,
		Error:       t.Error,
		Metrics:     t.Metrics,
		Check: GenericCheck{
			Description: v1.Description{
				Description:             t.Description,
				Name:                    t.Name,
				Icon:                    t.Icon,
				Labels:                  labels,
				TransformDeleteStrategy: t.TransformDeleteStrategy,
			},
			Type:     t.Type,
			Endpoint: t.Endpoint,
			CustomID: t.ID,
		},
	}
}

func (t TransformedCheckResult) GetDescription() string {
	return t.Description
}

type MetricType string

type Metric struct {
	Name   string            `json:"name,omitempty"`
	Type   MetricType        `json:"type,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value,omitempty"`
}

func (m Metric) ID() string {
	return fmt.Sprintf("%s-%s", m.Name, strings.Join(m.LabelNames(), "-"))
}

func (m Metric) LabelNames() []string {
	var names []string
	for k := range m.Labels {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func (m Metric) String() string {
	labels := ""
	if len(m.Labels) > 0 {
		labels = "{"
		for k, v := range m.Labels {
			if labels != "{" {
				labels += ", "
			}
			labels += fmt.Sprintf("%s=%s", k, v)
		}
		labels += "}"
	}
	return fmt.Sprintf("%s%s=%d", m.Name, labels, int(m.Value))
}

func (e Endpoint) GetEndpoint() string {
	return e.String
}
