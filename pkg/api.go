package pkg

import (
	"fmt"
	"time"

	"github.com/flanksource/commons/console"
)

type Endpointer interface {
	GetEndpoint() string
}

type Describable interface {
	GetDescription() string
}
type Endpoint struct {
	String string
}

type Config struct {
	HTTP       []HTTP        `yaml:"http,omitempty"`
	DNS        []DNS         `yaml:"dns,omitempty"`
	DockerPull []DockerPull  `yaml:"docker,omitempty"`
	DockerPush []DockerPush  `yaml:"dockerPush,omitempty"`
	S3         []S3          `yaml:"s3,omitempty"`
	S3Bucket   []S3Bucket    `yaml:"s3Bucket,omitempty"`
	TCP        []TCP         `yaml:"tcp,omitempty"`
	Pod        []Pod         `yaml:"pod,omitempty"`
	LDAP       []LDAP        `yaml:"ldap,omitempty"`
	SSL        []SSL         `yaml:"ssl,omitempty"`
	ICMP       []ICMP        `yaml:"icmp,omitempty"`
	Postgres   []Postgres    `yaml:"postgres,omitempty"`
	Helm       []Helm        `yaml:"helm,omitempty"`
	Interval   time.Duration `yaml:"-"`
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
	Pass        bool
	Invalid     bool
	Duration    int64
	Endpoint    string
	Description string
	Message     string
	Metrics     []Metric
	// Check is the configuration
	Check interface{}
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
	Description string `yaml:"description"`
}

func (e Endpoint) GetEndpoint() string {
	return e.String
}

func (c Check) GetDescription() string {
	return c.Description
}

type HTTPCheck struct {
	// HTTP endpoints to crawl
	Endpoints []string `yaml:"endpoints"`
	// Maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer.
	ThresholdMillis int `yaml:"thresholdMillis"`
	// Expected response codes for the HTTP Request.
	ResponseCodes []int `yaml:"responseCodes"`
	// Exact response content expected to be returned by the endpoint.
	ResponseContent string `yaml:"responseContent"`
	// Maximum number of days until the SSL Certificate expires.
	MaxSSLExpiry int `yaml:"maxSSLExpiry"`
}

type HTTPCheckResult struct {
	// Check is the configuration
	Check        interface{}
	Endpoint     string
	Record       string
	ResponseCode int
	SSLExpiry    int
	Content      string
	ResponseTime int64
}

func (check HTTPCheckResult) String() string {
	return fmt.Sprintf("%s ssl=%d code=%d time=%d", check.Endpoint, check.SSLExpiry, check.ResponseCode, check.ResponseTime)
}

type ICMPCheck struct {
	Description         string   `yaml:"description"`
	Endpoints           []string `yaml:"endpoints"`
	ThresholdMillis     float64  `yaml:"thresholdMillis"`
	PacketLossThreshold float64  `yaml:"packetLossThreshold"`
	PacketCount         int      `yaml:"packetCount"`
}

func (c ICMPCheck) GetDescription() string {
	return c.Description
}

type Bucket struct {
	Name     string `yaml:"name"`
	Region   string `yaml:"region"`
	Endpoint string `yaml:"endpoint"`
}

type S3Check struct {
	Description string   `yaml:"description"`
	Buckets     []Bucket `yaml:"buckets"`
	AccessKey   string   `yaml:"accessKey"`
	SecretKey   string   `yaml:"secretKey"`
	ObjectPath  string   `yaml:"objectPath"`
	// Skip TLS verify when connecting to s3
	SkipTLSVerify bool `yaml:"skipTLSVerify"`
}

func (c S3Check) GetDescription() string {
	return c.Description
}

func (c S3BucketCheck) GetDescription() string {
	return c.Description
}

type S3BucketCheck struct {
	Description string `yaml:"description"`
	Bucket      string `yaml:"bucket"`
	AccessKey   string `yaml:"accessKey"`
	SecretKey   string `yaml:"secretKey"`
	Region      string `yaml:"region"`
	Endpoint    string `yaml:"endpoint"`
	// glob path to restrict matches to a subset
	ObjectPath string `yaml:"objectPath"`
	ReadWrite  bool   `yaml:"readWrite"`
	// maximum allowed age of matched objects in seconds
	MaxAge int64 `yaml:"maxAge"`
	// min size of of most recent matched object in bytes
	MinSize int64 `yaml:"minSize"`
	// Use path style path: http://s3.amazonaws.com/BUCKET/KEY instead of http://BUCKET.s3.amazonaws.com/KEY
	UsePathStyle bool `yaml:"usePathStyle"`
	// Skip TLS verify when connecting to s3
	SkipTLSVerify bool `yaml:"skipTLSVerify"`
}

func (s3 S3BucketCheck) GetEndpoint() string {
	return s3.Bucket
}

type ICMPCheckResult struct {
	Description string `yaml:"description"`
	Endpoint    string
	Record      string
	Latency     float64
	PacketLoss  float64
}

type DNSCheckResult struct {
	Description string `yaml:"description"`
	LookupTime  string
	Records     string
}

type DockerPullCheck struct {
	Description    string `yaml:"description"`
	Image          string `yaml:"image"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	ExpectedDigest string `yaml:"expectedDigest"`
	ExpectedSize   int64  `yaml:"expectedSize"`
}

func (c DockerPullCheck) GetDescription() string {
	return c.Description
}

type DockerPushCheck struct {
	Description string `yaml:"description"`
	Image       string `yaml:"image"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
}

func (c DockerPushCheck) GetDescription() string {
	return c.Description
}

type PostgresCheck struct {
	Description string `yaml:"description"`
	Driver      string `yaml:"driver"`
	Connection  string `yaml:"connection"`
	Query       string `yaml:"query"`
	Result      int    `yaml:"results"`
}

func (c PostgresCheck) GetDescription() string {
	return c.Description
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

type PodCheck struct {
	Description          string `yaml:"description"`
	Name                 string `yaml:"name"`
	Namespace            string `yaml:"namespace"`
	Spec                 string `yaml:"spec"`
	ScheduleTimeout      int64  `yaml:"scheduleTimeout"`
	ReadyTimeout         int64  `yaml:"readyTimeout"`
	HttpTimeout          int64  `yaml:"httpTimeout"`
	DeleteTimeout        int64  `yaml:"deleteTimeout"`
	IngressTimeout       int64  `yaml:"ingressTimeout"`
	HttpRetryInterval    int64  `yaml:"httpRetryInterval"`
	Deadline             int64  `yaml:"deadline"`
	Port                 int32  `yaml:"port"`
	Path                 string `yaml:"path"`
	IngressName          string `yaml:"ingressName"`
	IngressHost          string `yaml:"ingressHost"`
	ExpectedContent      string `yaml:"expectedContent"`
	ExpectedHttpStatuses []int  `yaml:"expectedHttpStatuses"`
}

func (c PodCheck) GetDescription() string {
	return c.Description
}

func (p PodCheck) GetEndpoint() string {
	return p.Name
}

func (p PodCheck) String() string {
	return "pod/" + p.Name
}

type LDAPCheck struct {
	Description   string `yaml:"description"`
	Host          string `yaml:"host"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	BindDN        string `yaml:"bindDN"`
	UserSearch    string `yaml:"userSearch"`
	SkipTLSVerify bool   `yaml:"skipTLSVerify"`
}

func (c LDAPCheck) GetDescription() string {
	return c.Description
}

type DNSCheck struct {
	Description string   `yaml:"description"`
	Server      string   `yaml:"server"`
	Port        int      `yaml:"port"`
	Query       string   `yaml:"query,omitempty"`
	QueryType   string   `yaml:"querytype"`
	MinRecords  int      `yaml:"minrecords,omitempty"`
	ExactReply  []string `yaml:"exactreply,omitempty"`
	Timeout     int      `yaml:"timeout"`
	SrvReply    SrvReply `yaml:"srvReply,omitempty"`
}

func (c DNSCheck) GetDescription() string {
	return c.Description
}

type HelmCheck struct {
	Description string  `yaml:"description"`
	Chartmuseum string  `yaml:"chartmuseum"`
	Project     string  `yaml:"project,omitempty"`
	Username    string  `yaml:"username"`
	Password    string  `yaml:"password"`
	CaFile      *string `yaml:"cafile,omitempty"`
}

func (c HelmCheck) GetDescription() string {
	return c.Description
}

/*

```yaml
http:
  - endpoints:
      - https://httpstat.us/200
      - https://httpstat.us/301
    thresholdMillis: 3000
    responseCodes: [201,200,301]
    responseContent: ""
    maxSSLExpiry: 60
  - endpoints:
      - https://httpstat.us/500
    thresholdMillis: 3000
    responseCodes: [500]
    responseContent: ""
    maxSSLExpiry: 60
  - endpoints:
      - https://httpstat.us/500
    thresholdMillis: 3000
    responseCodes: [302]
    responseContent: ""
    maxSSLExpiry: 60
```
*/
type HTTP struct {
	HTTPCheck `yaml:",inline"`
}

type SSL struct {
	Check `yaml:",inline"`
}

/*

```yaml
dns:
  - server: 8.8.8.8
    port: 53
    query: "flanksource.com"
    querytype: "A"
    minrecords: 1
    exactreply: ["34.65.228.161"]
    timeout: 10
```
*/
type DNS struct {
	DNSCheck `yaml:",inline"`
}

/*
# Check docker images

This check will try to pull a Docker image from specified registry, verify it's checksum and size.

```yaml

docker:
  - image: docker.io/library/busybox:1.31.1
    username:
    password:
    expectedDigest: 6915be4043561d64e0ab0f8f098dc2ac48e077fe23f488ac24b665166898115a
    expectedSize: 1219782
```

*/
type DockerPull struct {
	DockerPullCheck `yaml:",inline"`
}

type DockerPush struct {
	DockerPushCheck `yaml:",inline"`
}

/*
This check will:

* list objects in the bucket to check for Read permissions
* PUT an object into the bucket for Write permissions
* download previous uploaded object to check for Get permissions

```yaml

s3:
  - buckets:
      - name: "test-bucket"
        region: "us-east-1"
        endpoint: "https://test-bucket.s3.us-east-1.amazonaws.com"
    secretKey: "<access-key>"
    accessKey: "<secret-key>"
    objectPath: "path/to/object"
```
*/
type S3 struct {
	S3Check `yaml:",inline"`
}

/*
This check will

- search objects matching the provided object path pattern
- check that latest object is no older than provided MaxAge value in seconds
- check that latest object size is not smaller than provided MinSize value in bytes.

```yaml
s3Bucket:
  - bucket: foo
    accessKey: "<access-key>"
    secretKey: "<secret-key>"
    region: "us-east-2"
    endpoint: "https://s3.us-east-2.amazonaws.com"
    objectPath: "(.*)archive.zip$"
    readWrite: true
    maxAge: 5000000
    minSize: 50000
```
*/
type S3Bucket struct {
	S3BucketCheck `yaml:",inline"`
}

type TCP struct {
	Check `yaml:",inline"`
}

/*
```yaml
pod:
  - name: golang
    namespace: default
    spec: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: hello-world-golang
        namespace: default
        labels:
          app: hello-world-golang
      spec:
        containers:
          - name: hello
            image: quay.io/toni0/hello-webserver-golang:latest
    port: 8080
    path: /foo/bar
    ingressName: hello-world-golang
    ingressHost: "hello-world-golang.127.0.0.1.nip.io"
    scheduleTimeout: 2000
    readyTimeout: 5000
    httpTimeout: 2000
    deleteTimeout: 12000
    ingressTimeout: 5000
    deadline: 29000
    httpRetryInterval: 200
    expectedContent: bar
    expectedHttpStatuses: [200, 201, 202]
```
*/
type Pod struct {
	PodCheck `yaml:",inline"`
}

/*

The LDAP check will:

* bind using provided user/password to the ldap host. Supports ldap/ldaps protocols.
* search an object type in the provided bind DN.s

```yaml

ldap:
  - host: ldap://127.0.0.1:10389
    username: uid=admin,ou=system
    password: secret
    bindDN: ou=users,dc=example,dc=com
    userSearch: "(&(objectClass=organizationalPerson))"
  - host: ldap://127.0.0.1:10389
    username: uid=admin,ou=system
    password: secret
    bindDN: ou=groups,dc=example,dc=com
    userSearch: "(&(objectClass=groupOfNames))"
```
*/
type LDAP struct {
	LDAPCheck `yaml:",inline"`
}

/*
This test will check ICMP packet loss and duration.

```yaml

icmp:
  - endpoints:
      - https://google.com
      - https://yahoo.com
    thresholdMillis: 400
    packetLossThreshold: 0.5
    packetCount: 2
```
*/
type ICMP struct {
	ICMPCheck `yaml:",inline"`
}

/*
This check will try to connect to a specified Postgresql database, run a query against it and verify the results.

```yaml

postgres:
  - connection: "user=postgres password=mysecretpassword host=192.168.0.103 port=15432 dbname=postgres sslmode=disable"
    query:  "SELECT 1"
		results: 1
```
*/
type Postgres struct {
	PostgresCheck `yaml:",inline"`
}

type Helm struct {
	HelmCheck `yaml:",inline"`
}

type SrvReply struct {
	Target   string `yaml:"target,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
	Weight   int    `yaml:"wight,omitempty"`
}
