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
	ICMP          []ICMP          `yaml:"icmp,omitempty"`
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
	Metrics  []Metric
}

type Metric struct {
	Name   string
	Type   MetricType
	Labels map[string]string
	Value  float64
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

type S3Check struct {
}

type ICMPCheckResult struct {
	Endpoint   string
	Record     string
	Latency    float64
	PacketLoss float64
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
	Check `yaml:",inline"`
}

type S3 struct {
	S3Check `yaml:",inline"`
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

type ICMP struct {
	ICMPCheck `yaml:",inline"`
}
