package pkg

type Config struct {
	HTTP          []HTTP          `yaml:"http,omitempty"`
	DNS           []DNS           `yaml:"dns,omitempty"`
	DockerPull    []DockerPull    `yaml:"docker,omitempty"`
	S3            []S3            `yaml:"s3,omitempty"`
	TCP           []TCP           `yaml:"tcp,omitempty"`
	Pod           []Pod           `yaml:"pod,omitempty"`
	PodAndIngress []PodAndIngress `yaml:"pod_and_ingress,omitempty"`
	LDAP          []LDAP          `yaml:"ldap,omitempty"`
	SSL           []SSL           `yaml:"ssl,omitempty"`
}

type Checker interface {
	Check(args map[string]interface{}) *CheckResult
}

type CheckResult struct {
	Pass    bool
	Invalid bool
	Metrics []Metric
}

type Metric struct {
}

type Check struct {
}

type HTTP struct {
	Check `yaml:",inline"`
}

type SSL struct {
	Check `yaml:",inline"`
}
type DNS struct {
	Check `yaml:",inline"`
}

type DockerPull struct {
	Check `yaml:",inline"`
}

type S3 struct {
	Check `yaml:",inline"`
}

type TCP struct {
	Check `yaml:",inline"`
}

type Pod struct {
	Check `yaml:",inline"`
}

type PodAndIngress struct {
	Check `yaml:",inline"`
}

type LDAP struct {
	Check `yaml:",inline"`
}

type PostgreSQL struct {
	Check `yaml:",inline"`
}
