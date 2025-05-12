package v1

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/commons/duration"
	"github.com/flanksource/duty/types"
	"github.com/flanksource/gomplate/v3"
	"github.com/invopop/jsonschema"
	"github.com/samber/lo"
	"github.com/timberio/go-datemath"
)

type Duration string

func (d Duration) GetHours() (*time.Duration, error) {
	v, err := duration.ParseDuration(string(d))
	if err != nil {
		return nil, err
	}
	_d := time.Duration(v.Hours()) * time.Hour
	return &_d, nil
}

func (d *Duration) GetDuration() (*time.Duration, error) {
	if d == nil {
		return nil, nil
	}
	_d, err := duration.ParseDuration(string(*d))
	return lo.ToPtr(time.Duration(_d)), err
}

func (d *Duration) GetDurationOrZero() (time.Duration, error) {
	return d.GetDurationOr(time.Duration(0))
}

// GetDuration parses a duration or returns the default if a nil value is passed or a parsing error occurs
func (d *Duration) GetDurationOr(def time.Duration) (time.Duration, error) {
	if d == nil {
		return def, nil
	}

	parsed, err := duration.ParseDuration(string(*d))
	if err != nil {
		return def, err
	}
	return time.Duration(parsed), nil
}

func (d *Duration) Validate() error {
	if _, err := d.GetDuration(); err != nil {
		return fmt.Errorf("invalid duration: %v", string(*d))
	}
	return nil
}

func (d Duration) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Description: "Duration e.g. 500ms, 2h, 2m",
	}
}

type Size string

func (s Size) String() string {
	var v datasize.ByteSize
	_ = v.UnmarshalText([]byte(s))
	return v.HumanReadable()
}

func (s Size) Value() (*int64, error) {
	var v datasize.ByteSize
	err := v.UnmarshalText([]byte(s))
	_v := int64(v.Bytes())
	return &_v, err
}

type FolderFilter struct {
	MinAge  Duration `yaml:"minAge,omitempty" json:"minAge,omitempty"`
	MaxAge  Duration `yaml:"maxAge,omitempty" json:"maxAge,omitempty"`
	Since   string   `yaml:"since,omitempty" json:"since,omitempty"`
	MinSize Size     `yaml:"minSize,omitempty" json:"minSize,omitempty"`
	MaxSize Size     `yaml:"maxSize,omitempty" json:"maxSize,omitempty"`
	Regex   string   `yaml:"regex,omitempty" json:"regex,omitempty"`
}

func (f FolderFilter) String() string {
	s := []string{}
	if f.MinAge != "" {
		s = append(s, fmt.Sprintf("minAge="+string(f.MinAge)))
	}
	if f.MaxAge != "" {
		s = append(s, "maxAge="+string(f.MaxAge))
	}
	if f.MinSize != "" {
		s = append(s, "minSize="+string(f.MinSize))
	}
	if f.MaxSize != "" {
		s = append(s, "maxSize="+string(f.MaxSize))
	}
	if f.Regex != "" {
		s = append(s, "regex="+f.Regex)
	}
	if f.Since != "" {
		s = append(s, "since="+f.Since)
	}
	return strings.Join(s, ", ")
}

// +k8s:deepcopy-gen=false
type FolderFilterContext struct {
	FolderFilter
	minAge, maxAge   *time.Duration
	minSize, maxSize *int64
	AllowDir         bool // Allow directories to support recursive folder checks
	Since            *time.Time
	// kubebuilder:object:generate=false
	regex *regexp.Regexp
}

func (f FolderFilter) New() (*FolderFilterContext, error) {
	ctx := &FolderFilterContext{}
	var err error

	if f.MaxAge != "" {
		d, err := f.MaxAge.GetHours()
		if err != nil {
			return nil, err
		}
		ctx.maxAge = d
	}
	if f.MinAge != "" {
		d, err := f.MinAge.GetHours()
		if err != nil {
			return nil, err
		}
		ctx.minAge = d
	}
	if f.Regex != "" {
		re, err := regexp.Compile(f.Regex)
		if err != nil {
			return nil, err
		}
		ctx.regex = re
	}
	if f.MinSize != "" {
		if ctx.minSize, err = f.MinSize.Value(); err != nil {
			return nil, err
		}
	}
	if f.MaxSize != "" {
		if ctx.maxSize, err = f.MaxSize.Value(); err != nil {
			return nil, err
		}
	}
	if f.Since != "" {
		if since, err := tryParse(f.Since); err == nil {
			ctx.Since = &since
		} else {
			if since, err := datemath.Parse(f.Since); err != nil {
				return nil, fmt.Errorf("could not parse since: %s: %v", f.Since, err)
			} else {
				t := since.Time()
				ctx.Since = &t
			}
		}
		// add 1 second to the since time so that last_result.newest.modified can be used as a since
		after := ctx.Since.Add(1 * time.Second)
		ctx.Since = &after
	}
	return ctx, nil
}

var RFC3339NanoWithoutTimezone = "2006-01-02T15:04:05.999999999"

func tryParse(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(RFC3339NanoWithoutTimezone, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse %s", s)
}

func (f *FolderFilterContext) Filter(i fs.FileInfo) bool {
	if i.IsDir() && !f.AllowDir {
		return false
	}
	if f.maxAge != nil && time.Since(i.ModTime()) > *f.maxAge {
		return false
	}
	if f.minAge != nil && time.Since(i.ModTime()) < *f.minAge {
		return false
	}
	if f.minSize != nil && i.Size() < *f.minSize {
		return false
	}
	if f.maxSize != nil && i.Size() > *f.maxSize {
		return false
	}
	if f.regex != nil && !f.regex.MatchString(i.Name()) {
		return false
	}
	if f.Since != nil && i.ModTime().Before(*f.Since) {
		return false
	}
	return true
}

type FolderTest struct {
	//MinAge the latest object should be older than defined age
	MinAge Duration `yaml:"minAge,omitempty" json:"minAge,omitempty"`
	//MaxAge the latest object should be younger than defined age
	MaxAge Duration `yaml:"maxAge,omitempty" json:"maxAge,omitempty"`
	//MinCount the minimum number of files inside the searchPath
	MinCount *int `yaml:"minCount,omitempty" json:"minCount,omitempty"`
	//MinCount the minimum number of files inside the searchPath
	MaxCount *int `yaml:"maxCount,omitempty" json:"maxCount,omitempty"`
	//MinSize of the files inside the searchPath
	MinSize Size `yaml:"minSize,omitempty" json:"minSize,omitempty"`
	//MaxSize of the files inside the searchPath
	MaxSize Size `yaml:"maxSize,omitempty" json:"maxSize,omitempty"`

	// AvailableSize present on the filesystem
	AvailableSize Size `yaml:"availableSize,omitempty" json:"availableSize,omitempty"`
	// TotalSize present on the filesystem
	TotalSize Size `yaml:"totalSize,omitempty" json:"totalSize,omitempty"`
}

func (f FolderTest) GetMinAge() (*time.Duration, error) {
	if f.MinAge == "" {
		return nil, nil
	}
	d, err := f.MinAge.GetHours()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (f FolderTest) GetMaxAge() (*time.Duration, error) {
	if f.MaxAge == "" {
		return nil, nil
	}
	d, err := f.MaxAge.GetHours()
	if err != nil {
		return nil, err
	}
	return d, nil
}

type JSONCheck struct {
	Path  string `yaml:"path" json:"path"`
	Value string `yaml:"value" json:"value"`
}

type Authentication struct {
	Username types.EnvVar `yaml:"username,omitempty" json:"username,omitempty"`
	Password types.EnvVar `yaml:"password,omitempty" json:"password,omitempty"`
}

func (auth Authentication) IsEmpty() bool {
	return auth.Username.IsEmpty() && auth.Password.IsEmpty()
}

func (auth Authentication) GetUsername() string {
	return auth.Username.ValueStatic
}

func (auth Authentication) GetPassword() string {
	return auth.Password.ValueStatic
}

func (auth Authentication) GetDomain() string {
	parts := strings.Split(auth.GetUsername(), "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

type Display struct {
	Template `yaml:",inline" json:",inline"`
}

func (d Display) GetDisplayTemplate() Template {
	return d.Template
}

type Test struct {
	Template `yaml:",inline" json:",inline"`
}

func (t Test) GetTestTemplate() Template {
	return t.Template
}

type Template struct {
	Template   string `yaml:"template,omitempty" json:"template,omitempty"`
	JSONPath   string `yaml:"jsonPath,omitempty" json:"jsonPath,omitempty"`
	Expression string `yaml:"expr,omitempty" json:"expr,omitempty"`
	Javascript string `yaml:"javascript,omitempty" json:"javascript,omitempty"`
}

func (t Template) IsEmpty() bool {
	return t.Template == "" && t.JSONPath == "" && t.Expression == "" && t.Javascript == ""
}

// Convert to gomplate.Template
func (t Template) Gomplate() gomplate.Template {
	return gomplate.Template{
		Template:   t.Template,
		JSONPath:   t.JSONPath,
		Expression: t.Expression,
		Javascript: t.Javascript,
	}
}

// +k8s:deepcopy-gen=false
type DisplayTemplate interface {
	GetDisplayTemplate() Template
}

// +k8s:deepcopy-gen=false
type TestFunction interface {
	GetTestFunction() Template
}

// +k8s:deepcopy-gen=false
type Transformer interface {
	GetTransformer() Template
}

type Templatable struct {
	Test      Template `yaml:"test,omitempty" json:"test,omitempty"`
	Display   Template `yaml:"display,omitempty" json:"display,omitempty"`
	Transform Template `yaml:"transform,omitempty" json:"transform,omitempty"`
}

func (t Templatable) GetTestFunction() Template {
	return t.Test
}

func (t Templatable) GetDisplayTemplate() Template {
	return t.Display
}

func (t Templatable) GetTransformer() Template {
	return t.Transform
}

type Labels map[string]string

func (l Labels) AddLabels(extra map[string]interface{}) map[string]string {
	var labels = make(map[string]string)
	for k, v := range l {
		labels[k] = v
	}
	for k, v := range extra {
		switch val := v.(type) {
		case string:
			labels[k] = val
		case int:
			labels[k] = strconv.Itoa(val)
		}
	}
	return labels
}

type Description struct {
	// Description for the check
	Description string `yaml:"description,omitempty" json:"description,omitempty" template:"true"`
	// Name of the check
	Name string `yaml:"name" json:"name" template:"true"`
	// TODO: namespace is a json.RawMessage for backwards compatibility when it used to be a resource selector
	// can be removed in a few versions time

	// Namespace to insert the check into, if different to the namespace the canary is defined, e.g.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=string
	Namespace json.RawMessage `yaml:"namespace,omitempty" json:"namespace,omitempty"  jsonschema:"type=string"`
	// Icon for overwriting default icon on the dashboard
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty" template:"true"`
	// Labels for the check
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`
	// Transformed checks have a delete strategy on deletion they can either be marked healthy, unhealthy or left as is
	TransformDeleteStrategy string `yaml:"transformDeleteStrategy,omitempty" json:"transformDeleteStrategy,omitempty"`
	// Metrics to expose from check.
	// https://canarychecker.io/concepts/metrics-exporter
	// +kubebuilder:validation:XPreserveUnknownFields
	Metrics []external.Metrics `json:"metrics,omitempty" yaml:"metrics,omitempty"`

	ResultLookup string `json:"result_lookup,omitempty" yaml:"result_lookup,omitempty"`
}

func (d Description) String() string {
	if d.Description != "" {
		return d.Description
	}
	return d.Name
}

func (d Description) GetDescription() string {
	return d.Description
}

func (d Description) GetIcon() string {
	return d.Icon
}

func (d Description) GetMetricsSpec() []external.Metrics {
	return d.Metrics
}

func (d Description) GetName() string {
	return d.Name
}

func (d Description) GetNamespace() string {
	s := string(d.Namespace)
	if s == "" || s == "{}" {
		return ""
	}
	if !strings.HasPrefix(s, "{") {
		return s
	}
	var r types.ResourceSelector
	if err := json.Unmarshal(d.Namespace, &r); err != nil {
		return ""
	}
	return r.Name
}

func (d Description) GetLabels() map[string]string {
	return d.Labels
}

func (d Description) GetTransformDeleteStrategy() string {
	return d.TransformDeleteStrategy
}

type Connection struct {
	// Connection name e.g. connection://http/google
	Connection string `yaml:"connection,omitempty" json:"connection,omitempty"`
	// Connection url, interpolated with username,password
	URL                  string `yaml:"url,omitempty" json:"url,omitempty" template:"true"`
	types.Authentication `yaml:",inline" json:",inline"`
}

func (c Connection) GetEndpoint() string {
	return SanitizeEndpoints(c.URL)
}

// Obfuscate passwords of the form ' password=xxxxx ' from connectionString since
// connectionStrings are used as metric labels and we don't want to leak passwords
// Returns the Connection string with the password replaced by '###'
func SanitizeEndpoints(connection string) string {
	connection = strings.TrimPrefix(connection, "git::")
	if _url, err := url.Parse(connection); err == nil {
		if _url.User != nil {
			_url.User = nil
			connection = _url.String()
		}
	}
	//looking for a substring that starts with a space,
	//'password=', then any non-whitespace characters,
	//until an ending space
	re := regexp.MustCompile(`password=([^;]*)`)
	return re.ReplaceAllString(connection, "password=###")
}
