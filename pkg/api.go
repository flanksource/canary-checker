package pkg

import (
	"fmt"

	"github.com/flanksource/commons/console"
)

type Config struct {
	HTTP          []HTTP          `yaml:"http,omitempty"`
	DNS           []DNS           `yaml:"dns,omitempty"`
	DockerPull    []DockerPull    `yaml:"docker,omitempty"`
	S3            []S3            `yaml:"s3,omitempty"`
	S3Bucket      []S3Bucket      `yaml:"s3Bucket,omitempty"`
	TCP           []TCP           `yaml:"tcp,omitempty"`
	Pod           []Pod           `yaml:"pod,omitempty"`
	PodAndIngress []PodAndIngress `yaml:"pod_and_ingress,omitempty"`
	LDAP          []LDAP          `yaml:"ldap,omitempty"`
	SSL           []SSL           `yaml:"ssl,omitempty"`
	ICMP          []ICMP          `yaml:"icmp,omitempty"`
	Postgres      []Postgres      `yaml:"postgres,omitempty"`
}

type Checker interface {
	CheckArgs(args map[string]interface{}) *CheckResult
}

// URL information
type URL struct {
	IP     string
	Port   int
	Host   string
	Scheme string
	Path   string
}

type CheckResult struct {
	Pass     bool
	Invalid  bool
	Duration int64
	Endpoint string
	Message  string
	Metrics  []Metric
}

func (c CheckResult) String() string {
	if c.Pass {
		return fmt.Sprintf("[%s] %s duration=%d %s %s", console.Greenf("PASS"), c.Endpoint, c.Duration, c.Metrics, c.Message)
	} else {
		if c.Invalid {
			return fmt.Sprintf("[%s] <%s> %s duration=%d %s %s", console.Redf("FAIL"), console.Redf("INVALID"), c.Endpoint, c.Duration, c.Metrics, c.Message)
		} else {
			return fmt.Sprintf("[%s] <%s> %s duration=%d %s %s", console.Redf("FAIL"), console.Greenf("VALID"), c.Endpoint, c.Duration, c.Metrics, c.Message)
		}
	}
}

type Metric struct {
	Name   string
	Type   MetricType
	Labels map[string]string
	Value  float64
}

func (m Metric) String() string {
	return fmt.Sprintf("%s=%d", m.Name, int(m.Value))
}

type Check struct {
}

type HTTPCheck struct {
	Endpoints       []string `yaml:"endpoints"`
	ThresholdMillis int      `yaml:"thresholdMillis"`
	ResponseCodes   []int    `yaml:"responseCodes"`
	ResponseContent string   `yaml:"responseContent"`
	MaxSSLExpiry    int      `yaml:"maxSSLExpiry"`
}

type HTTPCheckResult struct {
	Endpoint     string
	Record       string
	ResponseCode int
	SSLExpiry    int
	Content      string
	ResponseTime int64
}

type ICMPCheck struct {
	Endpoints           []string `yaml:"endpoints"`
	ThresholdMillis     float64  `yaml:"thresholdMillis"`
	PacketLossThreshold float64  `yaml:"packetLossThreshold"`
	PacketCount         int      `yaml:"packetCount"`
}

type Bucket struct {
	Name     string `yaml:"name"`
	Region   string `yaml:"region"`
	Endpoint string `yaml:"endpoint"`
}

type S3Check struct {
	Buckets    []Bucket `yaml:"buckets"`
	AccessKey  string   `yaml:"accessKey"`
	SecretKey  string   `yaml:"secretKey"`
	ObjectPath string   `yaml:"objectPath"`
}

type S3BucketCheck struct {
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	Region    string `yaml:"region"`
	Endpoint  string `yaml:"endpoint"`
	// glob path to restrict matches to a subet
	ObjectPath string `yaml:"objectPath"`
	ReadWrite  bool   `yaml:"readWrite"`
	// maximum allowed age of matched objects in seconds
	MaxAge int64 `yaml:"maxAge"`
	// min size of of most recent matched object in bytes
	MinSize int64 `yaml:"minSize"`
}

type ICMPCheckResult struct {
	Endpoint   string
	Record     string
	Latency    float64
	PacketLoss float64
}

type DockerPullCheck struct {
	Image          string `yaml:"image"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	ExpectedDigest string `yaml:"expectedDigest"`
	ExpectedSize   int64  `yaml:"expectedSize"`
}

type PostgresCheck struct {
	Driver     string `yaml:"driver"`
	Connection string `yaml:"connection"`
	Query      string `yaml:"query"`
	Result     int    `yaml:"results"`
}

// This is used to supply a default value for unsupplied fields
func (c *PostgresCheck) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawPostgresCheck PostgresCheck
	raw := rawPostgresCheck{
		Driver: "postgres",
		Query:  "SELECT 1",
		Result: 1,
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*c = PostgresCheck(raw)
	return nil
}
type LDAPCheck struct {
	Host       string `yaml:"host"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	BindDN     string `yaml:"bindDN"`
	UserSearch string `yaml:"userSearch"`
}

type HTTP struct {
	HTTPCheck `yaml:",inline"`
}

type SSL struct {
	Check `yaml:",inline"`
}

type DNS struct {
	Check `yaml:",inline"`
}

type DockerPull struct {
	DockerPullCheck `yaml:",inline"`
}

type S3 struct {
	S3Check `yaml:",inline"`
}

type S3Bucket struct {
	S3BucketCheck `yaml:",inline"`
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
	LDAPCheck `yaml:",inline"`
}

type PostgreSQL struct {
	Check `yaml:",inline"`
}

type ICMP struct {
	ICMPCheck `yaml:",inline"`
}

type Postgres struct {
	PostgresCheck `yaml:",inline"`
}
