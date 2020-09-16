package v1

import (
	"fmt"
	"regexp"

	"github.com/flanksource/canary-checker/api/external"
)

type HTTPCheck struct {
	Description string `yaml:"description" json:"description,omitempty"`
	// HTTP endpoint to crawl
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty"`
	// Maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer.
	ThresholdMillis int `yaml:"thresholdMillis" json:"thresholdMillis,omitempty"`
	// Expected response codes for the HTTP Request.
	ResponseCodes []int `yaml:"responseCodes" json:"responseCodes,omitempty"`
	// Exact response content expected to be returned by the endpoint.
	ResponseContent string `yaml:"responseContent" json:"responseContent,omitempty"`
	// Maximum number of days until the SSL Certificate expires.
	MaxSSLExpiry int `yaml:"maxSSLExpiry" json:"maxSSLExpiry,omitempty"`
}

func (c HTTPCheck) GetEndpoint() string {
	return c.Endpoint
}

func (c HTTPCheck) GetDescription() string {
	return c.Description
}

func (c HTTPCheck) GetType() string {
	return "http"
}

type SSLCheck struct {
	Description string `yaml:"description" json:"description,omitempty"`
	// HTTP endpoint to crawl
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty"`
	// Maximum number of days until the SSL Certificate expires.
	MaxSSLExpiry int `yaml:"maxSSLExpiry" json:"maxSSLExpiry,omitempty"`
}

func (c SSLCheck) GetEndpoint() string {
	return c.Endpoint
}

func (c SSLCheck) GetDescription() string {
	return c.Description
}

func (c SSLCheck) GetType() string {
	return "ssl"
}

type TCPCheck struct {
	Description     string `yaml:"description" json:"description,omitempty"`
	Endpoint        string `yaml:"endpoint" json:"endpoint,omitempty"`
	ThresholdMillis int64  `yaml:"thresholdMillis" json:"thresholdMillis,omitempty"`
}

func (t TCPCheck) GetEndpoint() string {
	return t.Endpoint
}

func (t TCPCheck) GetDescription() string {
	return t.Description
}

func (t TCPCheck) GetType() string {
	return "tcp"
}

type ICMPCheck struct {
	Description         string `yaml:"description" json:"description,omitempty"`
	Endpoint            string `yaml:"endpoint" json:"endpoint,omitempty"`
	ThresholdMillis     int64  `yaml:"thresholdMillis" json:"thresholdMillis,omitempty"`
	PacketLossThreshold int64  `yaml:"packetLossThreshold" json:"packetLossThreshold,omitempty"`
	PacketCount         int    `yaml:"packetCount" json:"packetCount,omitempty"`
}

func (c ICMPCheck) GetEndpoint() string {
	return c.Endpoint
}

func (c ICMPCheck) GetDescription() string {
	return c.Description
}

func (c ICMPCheck) GetType() string {
	return "icmp"
}

type Bucket struct {
	Name     string `yaml:"name" json:"name,omitempty"`
	Region   string `yaml:"region" json:"region,omitempty"`
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty"`
}

type S3Check struct {
	Description string `yaml:"description" json:"description,omitempty"`
	Bucket      Bucket `yaml:"bucket" json:"bucket,omitempty"`
	AccessKey   string `yaml:"accessKey" json:"accessKey,omitempty"`
	SecretKey   string `yaml:"secretKey" json:"secretKey,omitempty"`
	ObjectPath  string `yaml:"objectPath" json:"objectPath,omitempty"`
	// Skip TLS verify when connecting to s3
	SkipTLSVerify bool `yaml:"skipTLSVerify" json:"skipTLSVerify,omitempty"`
}

func (c S3Check) GetEndpoint() string {
	return fmt.Sprintf("%s/%s", c.Bucket.Endpoint, c.Bucket.Name)
}

func (c S3Check) GetDescription() string {
	return c.Description
}

func (c S3Check) GetType() string {
	return "s3"
}

type S3BucketCheck struct {
	Description string `yaml:"description" json:"description,omitempty"`
	Bucket      string `yaml:"bucket" json:"bucket,omitempty"`
	AccessKey   string `yaml:"accessKey" json:"accessKey,omitempty"`
	SecretKey   string `yaml:"secretKey" json:"secretKey,omitempty"`
	Region      string `yaml:"region" json:"region,omitempty"`
	Endpoint    string `yaml:"endpoint" json:"endpoint,omitempty"`
	// glob path to restrict matches to a subset
	ObjectPath string `yaml:"objectPath" json:"objectPath,omitempty"`
	ReadWrite  bool   `yaml:"readWrite" json:"readWrite,omitempty"`
	// maximum allowed age of matched objects in seconds
	MaxAge int64 `yaml:"maxAge" json:"maxAge,omitempty"`
	// min size of of most recent matched object in bytes
	MinSize int64 `yaml:"minSize" json:"minSize,omitempty"`
	// Use path style path: http://s3.amazonaws.com/BUCKET/KEY instead of http://BUCKET.s3.amazonaws.com/KEY
	UsePathStyle bool `yaml:"usePathStyle" json:"usePathStyle,omitempty"`
	// Skip TLS verify when connecting to s3
	SkipTLSVerify bool `yaml:"skipTLSVerify" json:"skipTLSVerify,omitempty"`
}

func (s3 S3BucketCheck) GetEndpoint() string {
	return fmt.Sprintf("%s/%s", s3.Endpoint, s3.Bucket)
}

func (c S3BucketCheck) GetDescription() string {
	return c.Description
}

func (c S3BucketCheck) GetType() string {
	return "s3Bucket"
}

type DockerPullCheck struct {
	Description    string `yaml:"description" json:"description,omitempty"`
	Image          string `yaml:"image" json:"image,omitempty"`
	Username       string `yaml:"username" json:"username,omitempty"`
	Password       string `yaml:"password" json:"password,omitempty"`
	ExpectedDigest string `yaml:"expectedDigest" json:"expectedDigest,omitempty"`
	ExpectedSize   int64  `yaml:"expectedSize" json:"expectedSize,omitempty"`
}

func (c DockerPullCheck) GetEndpoint() string {
	return c.Image
}

func (c DockerPullCheck) GetDescription() string {
	return c.Description
}

func (c DockerPullCheck) GetType() string {
	return "dockerPull"
}

type DockerPushCheck struct {
	Description string `yaml:"description" json:"description,omitempty"`
	Image       string `yaml:"image" json:"image,omitempty"`
	Username    string `yaml:"username" json:"username,omitempty"`
	Password    string `yaml:"password" json:"password,omitempty"`
}

func (c DockerPushCheck) GetEndpoint() string {
	return c.Image
}

func (c DockerPushCheck) GetDescription() string {
	return c.Description
}

func (c DockerPushCheck) GetType() string {
	return "dockerPush"
}

type ContainerdPullCheck struct {
	Description    string `yaml:"description" json:"description,omitempty"`
	Image          string `yaml:"image" json:"image,omitempty"`
	Username       string `yaml:"username" json:"username,omitempty"`
	Password       string `yaml:"password" json:"password,omitempty"`
	ExpectedDigest string `yaml:"expectedDigest" json:"expectedDigest,omitempty"`
	ExpectedSize   int64  `yaml:"expectedSize" json:"expectedSize,omitempty"`
}

func (c ContainerdPullCheck) GetEndpoint() string {
	return c.Image
}

func (c ContainerdPullCheck) GetDescription() string {
	return c.Description
}

func (c ContainerdPullCheck) GetType() string {
	return "containerdPull"
}

type ContainerdPushCheck struct {
	Description string `yaml:"description" json:"description,omitempty"`
	Image       string `yaml:"image" json:"image,omitempty"`
	Username    string `yaml:"username" json:"username,omitempty"`
	Password    string `yaml:"password" json:"password,omitempty"`
}

func (c ContainerdPushCheck) GetEndpoint() string {
	return c.Image
}

func (c ContainerdPushCheck) GetDescription() string {
	return c.Description
}

func (c ContainerdPushCheck) GetType() string {
	return "containerdPush"
}

type PostgresCheck struct {
	Description string `yaml:"description" json:"description,omitempty"`
	Driver      string `yaml:"driver" json:"driver,omitempty"`
	Connection  string `yaml:"connection" json:"connection,omitempty"`
	Query       string `yaml:"query" json:"query,omitempty"`
	Result      int    `yaml:"results" json:"result,omitempty"`
}

// Obfuscate passwords of the form ' password=xxxxx ' from connectionString since
// connectionStrings are used as metric labels and we don't want to leak passwords
// Returns the Connection string with the password replaced by '###'
func (c PostgresCheck) GetEndpoint() string {
	//looking for a substring that starts with a space,
	//'password=', then any non-whitespace characters,
	//until an ending space
	re := regexp.MustCompile(`\spassword=\S*\s`)
	return re.ReplaceAllString(c.Connection, " password=### ")
}

func (c PostgresCheck) GetDescription() string {
	return c.Description
}

func (c PostgresCheck) GetType() string {
	return "postgres"
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
	Description          string `yaml:"description" json:"description,omitempty"`
	Name                 string `yaml:"name" json:"name,omitempty"`
	Namespace            string `yaml:"namespace" json:"namespace,omitempty"`
	Spec                 string `yaml:"spec" json:"spec,omitempty"`
	ScheduleTimeout      int64  `yaml:"scheduleTimeout" json:"scheduleTimeout,omitempty"`
	ReadyTimeout         int64  `yaml:"readyTimeout" json:"readyTimeout,omitempty"`
	HttpTimeout          int64  `yaml:"httpTimeout" json:"httpTimeout,omitempty"`
	DeleteTimeout        int64  `yaml:"deleteTimeout" json:"deleteTimeout,omitempty"`
	IngressTimeout       int64  `yaml:"ingressTimeout" json:"ingressTimeout,omitempty"`
	HttpRetryInterval    int64  `yaml:"httpRetryInterval" json:"httpRetryInterval,omitempty"`
	Deadline             int64  `yaml:"deadline" json:"deadline,omitempty"`
	Port                 int64  `yaml:"port" json:"port,omitempty"`
	Path                 string `yaml:"path" json:"path,omitempty"`
	IngressName          string `yaml:"ingressName" json:"ingressName,omitempty"`
	IngressHost          string `yaml:"ingressHost" json:"ingressHost,omitempty"`
	ExpectedContent      string `yaml:"expectedContent" json:"expectedContent,omitempty"`
	ExpectedHttpStatuses []int  `yaml:"expectedHttpStatuses" json:"expectedHttpStatuses,omitempty"`
	PriorityClass        string `yaml:"priorityClass" json:"priorityClass,omitempty"`
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

func (c PodCheck) GetType() string {
	return "pod"
}

type LDAPCheck struct {
	Description   string `yaml:"description" json:"description,omitempty"`
	Host          string `yaml:"host" json:"host,omitempty"`
	Username      string `yaml:"username" json:"username,omitempty"`
	Password      string `yaml:"password" json:"password,omitempty"`
	BindDN        string `yaml:"bindDN" json:"bindDN,omitempty"`
	UserSearch    string `yaml:"userSearch" json:"userSearch,omitempty"`
	SkipTLSVerify bool   `yaml:"skipTLSVerify" json:"skipTLSVerify,omitempty"`
}

func (c LDAPCheck) GetEndpoint() string {
	return c.Host
}

func (c LDAPCheck) GetDescription() string {
	return c.Description
}

func (c LDAPCheck) GetType() string {
	return "ldap"
}

type NamespaceCheck struct {
	Description          string            `yaml:"description" json:"description,omitempty"`
	CheckName            string            `yaml:"checkName" json:"checkName,omitempty"`
	NamespaceNamePrefix  string            `yaml:"namespaceNamePrefix" json:"namespaceNamePrefix,omitempty"`
	NamespaceLabels      map[string]string `yaml:"namespaceLabels" json:"namespaceLabels,omitempty"`
	NamespaceAnnotations map[string]string `yaml:"namespaceAnnotations" json:"namespaceAnnotations,omitempty"`
	PodSpec              string            `yaml:"podSpec" json:"podSpec,omitempty"`
	ScheduleTimeout      int64             `yaml:"scheduleTimeout" json:"schedule_timeout,omitempty"`
	ReadyTimeout         int64             `yaml:"readyTimeout" json:"readyTimeout,omitempty"`
	HttpTimeout          int64             `yaml:"httpTimeout" json:"httpTimeout,omitempty"`
	DeleteTimeout        int64             `yaml:"deleteTimeout" json:"deleteTimeout,omitempty"`
	IngressTimeout       int64             `yaml:"ingressTimeout" json:"ingressTimeout,omitempty"`
	HttpRetryInterval    int64             `yaml:"httpRetryInterval" json:"httpRetryInterval,omitempty"`
	Deadline             int64             `yaml:"deadline" json:"deadline,omitempty"`
	Port                 int64             `yaml:"port" json:"port,omitempty"`
	Path                 string            `yaml:"path" json:"path,omitempty"`
	IngressName          string            `yaml:"ingressName" json:"ingressName,omitempty"`
	IngressHost          string            `yaml:"ingressHost" json:"ingressHost,omitempty"`
	ExpectedContent      string            `yaml:"expectedContent" json:"expectedContent,omitempty"`
	ExpectedHttpStatuses []int64           `yaml:"expectedHttpStatuses" json:"expectedHttpStatuses,omitempty"`
	PriorityClass        string            `yaml:"priorityClass" json:"priorityClass,omitempty"`
}

func (c NamespaceCheck) GetDescription() string {
	return c.Description
}

func (p NamespaceCheck) GetEndpoint() string {
	return p.CheckName
}

func (p NamespaceCheck) String() string {
	return "namespace/" + p.CheckName
}

func (c NamespaceCheck) GetType() string {
	return "namespace"
}

type DNSCheck struct {
	Description     string   `yaml:"description" json:"description,omitempty"`
	Server          string   `yaml:"server" json:"server,omitempty"`
	Port            int      `yaml:"port" json:"port,omitempty"`
	Query           string   `yaml:"query,omitempty" json:"query,omitempty"`
	QueryType       string   `yaml:"querytype" json:"querytype,omitempty"`
	MinRecords      int      `yaml:"minrecords,omitempty" json:"minrecords,omitempty"`
	ExactReply      []string `yaml:"exactreply,omitempty" json:"exactreply,omitempty"`
	Timeout         int      `yaml:"timeout" json:"timeout,omitempty"`
	ThresholdMillis int      `yaml:"thresholdMillis" json:"thresholdMillis,omitempty"`
	// SrvReply    SrvReply `yaml:"srvReply,omitempty" json:"srvReply,omitempty"`
}

func (c DNSCheck) GetEndpoint() string {
	return fmt.Sprintf("%s/%s@%s:%d", c.QueryType, c.Query, c.Server, c.Port)
}

func (c DNSCheck) GetDescription() string {
	return c.Description
}

func (c DNSCheck) GetType() string {
	return "dns"
}

type HelmCheck struct {
	Description string  `yaml:"description" json:"description,omitempty"`
	Chartmuseum string  `yaml:"chartmuseum" json:"chartmuseum,omitempty"`
	Project     string  `yaml:"project,omitempty" json:"project,omitempty"`
	Username    string  `yaml:"username" json:"username,omitempty"`
	Password    string  `yaml:"password" json:"password,omitempty"`
	CaFile      *string `yaml:"cafile,omitempty" json:"cafile,omitempty"`
}

func (c HelmCheck) GetEndpoint() string {
	return fmt.Sprintf("%s/%s", c.Chartmuseum, c.Project)
}

func (c HelmCheck) GetDescription() string {
	return c.Description
}

func (c HelmCheck) GetType() string {
	return "helm"
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
	HTTPCheck `yaml:",inline" json:"inline"`
}

type SSL struct {
	SSLCheck `yaml:",inline" json:"inline"`
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
	DNSCheck `yaml:",inline" json:"inline"`
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
	DockerPullCheck `yaml:",inline" json:"inline"`
}

type DockerPush struct {
	DockerPushCheck `yaml:",inline" json:"inline"`
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
	S3Check `yaml:",inline" json:"inline"`
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
	S3BucketCheck `yaml:",inline" json:"inline"`
}

type TCP struct {
	TCPCheck `yaml:",inline" json:"inline"`
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
	PodCheck `yaml:",inline" json:"inline"`
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
	LDAPCheck `yaml:",inline" json:"inline"`
}

/*

The Namespace check will:

* create a new namespace using the labels/annotations provided

```yaml

namespace:
  - namePrefix: "test-name-prefix-"
		labels:
			team: test
		annotations:
			"foo.baz.com/foo": "bar"
```
*/
type Namespace struct {
	NamespaceCheck `yaml:",inline" json:"inline"`
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
	ICMPCheck `yaml:",inline" json:"inline"`
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
	PostgresCheck `yaml:",inline" json:"inline"`
}

type Helm struct {
	HelmCheck `yaml:",inline" json:"inline"`
}

type SrvReply struct {
	Target   string `yaml:"target,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
	Weight   int    `yaml:"wight,omitempty"`
}

var AllChecks = []external.Check{
	HTTPCheck{},
	SSLCheck{},
	TCPCheck{},
	ICMPCheck{},
	S3Check{},
	S3BucketCheck{},
	DockerPullCheck{},
	DockerPushCheck{},
	ContainerdPullCheck{},
	ContainerdPushCheck{},
	PostgresCheck{},
	PodCheck{},
	LDAPCheck{},
	NamespaceCheck{},
	DNSCheck{},
	HelmCheck{},
}
