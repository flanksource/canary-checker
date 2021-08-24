package v1

import (
	"regexp"
	"strings"
	"time"

	"github.com/flanksource/kommons"
)

type FolderTest struct {
	//MinAge the latest object should be older than defined age
	MinAge *string `yaml:"minAge,omitempty" json:"minAge,omitempty"`
	//MaxAge the latest object should be younger than defined age
	MaxAge *string `yaml:"maxAge,omitempty" json:"maxAge,omitempty"`
	//MinCount the minimum number of files inside the searchPath
	MinCount *int `yaml:"minCount,omitempty" json:"minCount,omitempty"`
	//MinCount the minimum number of files inside the searchPath
	MaxCount *int `yaml:"maxCount,omitempty" json:"maxCount,omitempty"`
	//MinSize of the files inside the searchPath
	MinSize *int64 `yaml:"minSize,omitempty" json:"minSize,omitempty"`
	//MaxSize of the files inside the searchPath
	MaxSize *int64 `yaml:"maxSize,omitempty" json:"maxSize,omitempty"`
}

func (f FolderTest) GetMinAge() *time.Duration {
	if f.MinAge == nil {
		return nil
	}
	d, err := time.ParseDuration(*f.MinAge)
	if err != nil {
		return nil
	}
	return &d
}

func (f FolderTest) GetMaxAge() *time.Duration {
	if f.MaxAge == nil {
		return nil
	}
	d, err := time.ParseDuration(*f.MaxAge)
	if err != nil {
		return nil
	}
	return &d
}

type JSONCheck struct {
	Path  string `yaml:"path" json:"path"`
	Value string `yaml:"value" json:"value"`
}

type Authentication struct {
	Username kommons.EnvVar `yaml:"username" json:"username"`
	Password kommons.EnvVar `yaml:"password" json:"password"`
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
	Template string `yaml:"template,omitempty" json:"template,omitempty"`
	JSONPath string `yaml:"jsonPath,omitempty" json:"jsonPath,omitempty"`
}

// +k8s:deepcopy-gen=false
type DisplayTemplate interface {
	GetDisplayTemplate() Template
}

// +k8s:deepcopy-gen=false
type TestFunction interface {
	GetTestFunction() Template
}

type Templatable struct {
	Test    Template `yaml:"test,omitempty" json:"test,omitempty"`
	Display Template `yaml:"display,omitempty" json:"display,omitempty"`
}

func (t Templatable) GetTestFunction() Template {
	return t.Test
}

func (t Templatable) GetDisplayTemplate() Template {
	return t.Display
}

type Description struct {
	// Description for the check
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Name of the check
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// Icon for overwriting default icon on the dashboard
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

func (d Description) GetDescription() string {
	return d.Description
}

func (d Description) GetIcon() string {
	return d.Icon
}

// Obfuscate passwords of the form ' password=xxxxx ' from connectionString since
// connectionStrings are used as metric labels and we don't want to leak passwords
// Returns the Connection string with the password replaced by '###'

func sanitizeEndpoints(connection string) string {
	//looking for a substring that starts with a space,
	//'password=', then any non-whitespace characters,
	//until an ending space
	re := regexp.MustCompile(`\spassword=\S*\s`)
	return re.ReplaceAllString(connection, " password=### ")
}
