package v1

import (
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/flanksource/commons/duration"
	"github.com/flanksource/kommons"
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
	MinSize Size     `yaml:"minSize,omitempty" json:"minSize,omitempty"`
	MaxSize Size     `yaml:"maxSize,omitempty" json:"maxSize,omitempty"`
	Regex   string   `yaml:"regex,omitempty" json:"regex,omitempty"`
}

// +k8s:deepcopy-gen=false
type FolderFilterContext struct {
	FolderFilter
	minAge, maxAge   *time.Duration
	minSize, maxSize *int64
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
	return ctx, nil
}

func (f *FolderFilterContext) Filter(i os.FileInfo) bool {
	if i.IsDir() {
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
	Username kommons.EnvVar `yaml:"username" json:"username"`
	Password kommons.EnvVar `yaml:"password" json:"password"`
}

func (auth Authentication) IsEmpty() bool {
	return auth.Username.IsEmpty() && auth.Password.IsEmpty()
}

func (auth Authentication) GetUsername() string {
	return auth.Username.Value
}

func (auth Authentication) GetPassword() string {
	return auth.Password.Value
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

type Description struct {
	// Description for the check
	Description string `yaml:"description,omitempty" json:"description,omitempty" template:"true"`
	// Name of the check
	Name string `yaml:"name" json:"name" template:"true"`
	// Icon for overwriting default icon on the dashboard
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty" template:"true"`
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

func (d Description) GetName() string {
	return d.Name
}

type Connection struct {
	Connection     string         `yaml:"connection" json:"connection" template:"true"`
	Authentication Authentication `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// +k8s:deepcopy-gen=false
type Connectable interface {
	GetConnection() string
}

func (c Connection) GetConnection() string {
	return c.Connection
}

func (c Connection) GetEndpoint() string {
	return sanitizeEndpoints(c.Connection)
}

// Obfuscate passwords of the form ' password=xxxxx ' from connectionString since
// connectionStrings are used as metric labels and we don't want to leak passwords
// Returns the Connection string with the password replaced by '###'
func sanitizeEndpoints(connection string) string {
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
