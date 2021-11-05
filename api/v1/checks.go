package v1

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/kommons"
	v1 "k8s.io/api/core/v1"
)

type Check struct {
	Name, Type, Endpoint, Description, Icon string
}

func (c Check) GetType() string {
	return c.Type
}

func (c Check) GetEndpoint() string {
	return c.Endpoint
}

func (c Check) GetDescription() string {
	return c.Description
}

func (c Check) GetIcon() string {
	return c.Icon
}

func (c Check) GetName() string {
	return c.Name
}

type HTTPCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	// HTTP endpoint to check.  Mutually exclusive with Namespace
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty" template:"true"`
	// Namespace to crawl for TLS endpoints.  Mutually exclusive with Endpoint
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" template:"true"`
	// Maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer.
	ThresholdMillis int `yaml:"thresholdMillis,omitempty" json:"thresholdMillis,omitempty"`
	// Expected response codes for the HTTP Request.
	ResponseCodes []int `yaml:"responseCodes,omitempty" json:"responseCodes,omitempty"`
	// Exact response content expected to be returned by the endpoint.
	ResponseContent string `yaml:"responseContent,omitempty" json:"responseContent,omitempty"`
	// Path and value to of expect JSON response by the endpoint
	ResponseJSONContent JSONCheck `yaml:"responseJSONContent,omitempty" json:"responseJSONContent,omitempty"`
	// Maximum number of days until the SSL Certificate expires.
	MaxSSLExpiry int `yaml:"maxSSLExpiry,omitempty" json:"maxSSLExpiry,omitempty"`
	// Method to use - defaults to GET
	Method string `yaml:"method,omitempty" json:"method,omitempty"`
	// NTLM when set to true will do authentication using NTLM v1 protocol
	NTLM bool `yaml:"ntlm,omitempty" json:"ntlm,omitempty"`
	// NTLM when set to true will do authentication using NTLM v2 protocol
	NTLMv2 bool `yaml:"ntlmv2,omitempty" json:"ntlmv2,omitempty"`
	// Request Body Contents
	Body string `yaml:"body,omitempty" json:"body,omitempty" template:"true"`
	// Header fields to be used in the query
	Headers []kommons.EnvVar `yaml:"headers,omitempty" json:"headers,omitempty"`
	// Credentials for authentication headers
	Authentication *Authentication `yaml:"authentication,omitempty" json:"authentication,omitempty"`
}

func (c HTTPCheck) GetEndpoint() string {
	return c.Endpoint
}

func (c HTTPCheck) GetType() string {
	return "http"
}

func (c HTTPCheck) GetMethod() string {
	if c.Method != "" {
		return c.Method
	}
	return "GET"
}

type TCPCheck struct {
	Description     `yaml:",inline" json:",inline"`
	Endpoint        string `yaml:"endpoint" json:"endpoint,omitempty"`
	ThresholdMillis int64  `yaml:"thresholdMillis,omitempty" json:"thresholdMillis,omitempty"`
}

func (t TCPCheck) GetEndpoint() string {
	return t.Endpoint
}

func (t TCPCheck) GetType() string {
	return "tcp"
}

type ICMPCheck struct {
	Description         `yaml:",inline" json:",inline"`
	Endpoint            string `yaml:"endpoint" json:"endpoint,omitempty"`
	ThresholdMillis     int64  `yaml:"thresholdMillis,omitempty" json:"thresholdMillis,omitempty"`
	PacketLossThreshold int64  `yaml:"packetLossThreshold,omitempty" json:"packetLossThreshold,omitempty"`
	PacketCount         int    `yaml:"packetCount,omitempty" json:"packetCount,omitempty"`
}

func (c ICMPCheck) GetEndpoint() string {
	return c.Endpoint
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
	Description `yaml:",inline" json:",inline"`
	Bucket      Bucket `yaml:"bucket" json:"bucket,omitempty"`
	AccessKey   string `yaml:"accessKey" json:"accessKey,omitempty"`
	SecretKey   string `yaml:"secretKey" json:"secretKey,omitempty"`
	ObjectPath  string `yaml:"objectPath" json:"objectPath,omitempty"`
	// Skip TLS verify when connecting to s3
	SkipTLSVerify bool `yaml:"skipTLSVerify,omitempty" json:"skipTLSVerify,omitempty"`
}

func (c S3Check) GetEndpoint() string {
	return fmt.Sprintf("%s/%s", c.Bucket.Endpoint, c.Bucket.Name)
}

func (c S3Check) GetType() string {
	return "s3"
}

type CloudWatchCheck struct {
	Description   `yaml:",inline" json:",inline"`
	AWSConnection `yaml:",inline" json:",inline"`
	Templatable   `yaml:",inline" json:",inline"`
	Filter        CloudWatchFilter `yaml:"filter,omitempty" json:"filter,omitempty"`
}

type CloudWatchFilter struct {
	ActionPrefix *string  `yaml:"actionPrefix,omitempty" json:"actionPrefix,omitempty"`
	AlarmPrefix  *string  `yaml:"alarmPrefix,omitempty" json:"alarmPrefix,omitempty"`
	Alarms       []string `yaml:"alarms,omitempty" json:"alarms,omitempty"`
	State        string   `yaml:"state,omitempty" json:"state,omitempty"`
}

func (c CloudWatchCheck) GetEndpoint() string {
	endpoint := c.Region
	if c.Filter.ActionPrefix != nil {
		endpoint += "-" + *c.Filter.ActionPrefix
	}
	if c.Filter.AlarmPrefix != nil {
		endpoint += "-" + *c.Filter.AlarmPrefix
	}
	return endpoint
}

func (c CloudWatchCheck) GetType() string {
	return "cloudwatch"
}

type ResticCheck struct {
	Description `yaml:",inline" json:",inline"`
	// Repository The restic repository path eg: rest:https://user:pass@host:8000/ or rest:https://host:8000/ or s3:s3.amazonaws.com/bucket_name
	Repository string `yaml:"repository" json:"repository"`
	// Password for the restic repository
	Password *kommons.EnvVar `yaml:"password" json:"password"`
	// MaxAge for backup freshness
	MaxAge string `yaml:"maxAge" json:"maxAge"`
	// CheckIntegrity when enabled will check the Integrity and consistency of the restic reposiotry
	CheckIntegrity bool `yaml:"checkIntegrity,omitempty" json:"checkIntegrity,omitempty"`
	// AccessKey access key id for connection with aws s3, minio, wasabi, alibaba oss
	AccessKey *kommons.EnvVar `yaml:"accessKey,omitempty" json:"accessKey,omitempty"`
	// SecretKey secret access key for connection with aws s3, minio, wasabi, alibaba oss
	SecretKey *kommons.EnvVar `yaml:"secretKey,omitempty" json:"secretKey,omitempty"`
	// CaCert path to the root cert. In case of self-signed certificates
	CaCert string `yaml:"caCert,omitempty" json:"caCert,omitempty"`
}

func (c ResticCheck) GetEndpoint() string {
	return c.Repository
}

func (c ResticCheck) GetType() string {
	return "restic"
}

type JmeterCheck struct {
	Description `yaml:",inline" json:",inline"`
	// Jmx defines tge ConfigMap or Secret reference to get the JMX test plan
	Jmx kommons.EnvVar `yaml:"jmx" json:"jmx"`
	// Host is the server against which test plan needs to be executed
	Host string `yaml:"host,omitempty" json:"host,omitempty"`
	// Port on which the server is running
	Port int32 `yaml:"port,omitempty" json:"port,omitempty"`
	// Properties defines the local Jmeter properties
	Properties []string `yaml:"properties,omitempty" json:"properties,omitempty"`
	// SystemProperties defines the java system property
	SystemProperties []string `yaml:"systemProperties,omitempty" json:"systemProperties,omitempty"`
	// ResponseDuration under which the all the test should pass
	ResponseDuration string `yaml:"responseDuration,omitempty" json:"responseDuration,omitempty"`
}

func (c JmeterCheck) GetEndpoint() string {
	return fmt.Sprintf(c.Host + ":" + string(c.Port))
}

func (c JmeterCheck) GetType() string {
	return "jmeter"
}

type DockerPullCheck struct {
	Description    `yaml:",inline" json:",inline"`
	Image          string          `yaml:"image" json:"image"`
	Auth           *Authentication `yaml:"auth,omitempty" json:"auth,omitempty"`
	ExpectedDigest string          `yaml:"expectedDigest" json:"expectedDigest,omitempty"`
	ExpectedSize   int64           `yaml:"expectedSize" json:"expectedSize,omitempty"`
}

func (c DockerPullCheck) GetEndpoint() string {
	return c.Image
}

func (c DockerPullCheck) GetType() string {
	return "dockerPull"
}

type DockerPushCheck struct {
	Description `yaml:",inline" json:",inline"`
	Image       string          `yaml:"image" json:"image"`
	Auth        *Authentication `yaml:"auth,omitempty" json:"auth,omitempty"`
}

func (c DockerPushCheck) GetEndpoint() string {
	return c.Image
}

func (c DockerPushCheck) GetType() string {
	return "dockerPush"
}

type ContainerdPullCheck struct {
	Description    `yaml:",inline" json:",inline"`
	Image          string         `yaml:"image" json:"image"`
	Auth           Authentication `yaml:"auth,omitempty" json:"auth,omitempty"`
	ExpectedDigest string         `yaml:"expectedDigest,omitempty" json:"expectedDigest,omitempty"`
	ExpectedSize   int64          `yaml:"expectedSize,omitempty" json:"expectedSize,omitempty"`
}

func (c ContainerdPullCheck) GetEndpoint() string {
	return c.Image
}

func (c ContainerdPullCheck) GetType() string {
	return "containerdPull"
}

type ContainerdPushCheck struct {
	Description `yaml:",inline" json:",inline"`
	Image       string `yaml:"image" json:"image"`
	Username    string `yaml:"username" json:"username,omitempty"`
	Password    string `yaml:"password" json:"password,omitempty"`
}

func (c ContainerdPushCheck) GetEndpoint() string {
	return c.Image
}

func (c ContainerdPushCheck) GetType() string {
	return "containerdPush"
}

type RedisCheck struct {
	Description `yaml:",inline" json:",inline"`
	Addr        string          `yaml:"addr" json:"addr" template:"true"`
	Auth        *Authentication `yaml:"auth,omitempty" json:"auth,omitempty"`
	DB          int             `yaml:"db" json:"db"`
}

func (c RedisCheck) GetType() string {
	return "redis"
}

func (c RedisCheck) GetEndpoint() string {
	return c.Addr
}

type SQLCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
	driver      string `yaml:"-" json:"-"`
	Query       string `yaml:"query" json:"query,omitempty" template:"true"`
	// Number rows to check for
	Result int `yaml:"results" json:"results,omitempty"`
}

func (c *SQLCheck) GetQuery() string {
	if c.Query == "" {
		return "SELECT 1"
	}
	return c.Query
}

func (c SQLCheck) GetDriver() string {
	return c.driver
}

func (c *SQLCheck) SetDriver(driver string) {
	c.driver = driver
}

func (c SQLCheck) GetType() string {
	return c.GetDriver()
}

func (c SQLCheck) GetEndpoint() string {
	if c.Name != "" {
		return c.Name
	}
	if c.Description.Description != "" {
		return c.Description.Description
	}
	return c.Connection.GetEndpoint()
}

type PostgresCheck struct {
	SQLCheck `yaml:",inline" json:",inline"`
}

func (p PostgresCheck) GetType() string {
	return "postgres"
}

type MssqlCheck struct {
	SQLCheck `yaml:",inline" json:",inline"`
}

/*
[include:datasources/mongo_pass.yaml]
*/
type Mongo struct {
	MongoDBCheck `yaml:",inline" json:",inline"`
}

func (m MssqlCheck) GetType() string {
	return "mssql"
}

type PodCheck struct {
	Description          `yaml:",inline" json:",inline"`
	Namespace            string `yaml:"namespace" json:"namespace,omitempty" template:"true"`
	Spec                 string `yaml:"spec" json:"spec,omitempty"`
	ScheduleTimeout      int64  `yaml:"scheduleTimeout,omitempty" json:"scheduleTimeout,omitempty"`
	ReadyTimeout         int64  `yaml:"readyTimeout,omitempty" json:"readyTimeout,omitempty"`
	HTTPTimeout          int64  `yaml:"httpTimeout,omitempty" json:"httpTimeout,omitempty"`
	DeleteTimeout        int64  `yaml:"deleteTimeout,omitempty" json:"deleteTimeout,omitempty"`
	IngressTimeout       int64  `yaml:"ingressTimeout,omitempty" json:"ingressTimeout,omitempty"`
	HTTPRetryInterval    int64  `yaml:"httpRetryInterval,omitempty" json:"httpRetryInterval,omitempty"`
	Deadline             int64  `yaml:"deadline,omitempty" json:"deadline,omitempty"`
	Port                 int64  `yaml:"port,omitempty" json:"port,omitempty"`
	Path                 string `yaml:"path,omitempty" json:"path,omitempty" template:"true"`
	IngressName          string `yaml:"ingressName" json:"ingressName,omitempty" template:"true" `
	IngressHost          string `yaml:"ingressHost" json:"ingressHost,omitempty" template:"true"`
	ExpectedContent      string `yaml:"expectedContent,omitempty" json:"expectedContent,omitempty" template:"true"`
	ExpectedHTTPStatuses []int  `yaml:"expectedHttpStatuses,omitempty" json:"expectedHttpStatuses,omitempty"`
	PriorityClass        string `yaml:"priorityClass,omitempty" json:"priorityClass,omitempty"`
}

func (c PodCheck) GetEndpoint() string {
	return c.Name
}

func (c PodCheck) String() string {
	return "pod/" + c.Name
}

func (c PodCheck) GetType() string {
	return "pod"
}

type LDAPCheck struct {
	Description   `yaml:",inline" json:",inline"`
	Host          string          `yaml:"host" json:"host" template:"true"`
	Auth          *Authentication `yaml:"auth" json:"auth"`
	BindDN        string          `yaml:"bindDN" json:"bindDN"`
	UserSearch    string          `yaml:"userSearch,omitempty" json:"userSearch,omitempty"`
	SkipTLSVerify bool            `yaml:"skipTLSVerify,omitempty" json:"skipTLSVerify,omitempty"`
}

func (c LDAPCheck) GetEndpoint() string {
	return c.Host
}

func (c LDAPCheck) GetType() string {
	return "ldap"
}

type NamespaceCheck struct {
	Description          `yaml:",inline" json:",inline"`
	CheckName            string            `yaml:"checkName" json:"checkName,omitempty" template:"true"`
	NamespaceNamePrefix  string            `yaml:"namespaceNamePrefix,omitempty" json:"namespaceNamePrefix,omitempty"`
	NamespaceLabels      map[string]string `yaml:"namespaceLabels,omitempty" json:"namespaceLabels,omitempty"`
	NamespaceAnnotations map[string]string `yaml:"namespaceAnnotations,omitempty" json:"namespaceAnnotations,omitempty"`
	PodSpec              string            `yaml:"podSpec" json:"podSpec"`
	ScheduleTimeout      int64             `yaml:"scheduleTimeout,omitempty" json:"schedule_timeout,omitempty"`
	ReadyTimeout         int64             `yaml:"readyTimeout,omitempty" json:"readyTimeout,omitempty"`
	HTTPTimeout          int64             `yaml:"httpTimeout,omitempty" json:"httpTimeout,omitempty"`
	DeleteTimeout        int64             `yaml:"deleteTimeout,omitempty" json:"deleteTimeout,omitempty"`
	IngressTimeout       int64             `yaml:"ingressTimeout,omitempty" json:"ingressTimeout,omitempty"`
	HTTPRetryInterval    int64             `yaml:"httpRetryInterval,omitempty" json:"httpRetryInterval,omitempty"`
	Deadline             int64             `yaml:"deadline,omitempty" json:"deadline,omitempty"`
	Port                 int64             `yaml:"port,omitempty" json:"port,omitempty"`
	Path                 string            `yaml:"path,omitempty" json:"path,omitempty"`
	IngressName          string            `yaml:"ingressName,omitempty" json:"ingressName,omitempty" template:"true"`
	IngressHost          string            `yaml:"ingressHost,omitempty" json:"ingressHost,omitempty" template:"true"`
	ExpectedContent      string            `yaml:"expectedContent,omitempty" json:"expectedContent,omitempty" template:"true"`
	ExpectedHTTPStatuses []int64           `yaml:"expectedHttpStatuses,omitempty" json:"expectedHttpStatuses,omitempty"`
	PriorityClass        string            `yaml:"priorityClass,omitempty" json:"priorityClass,omitempty"`
}

func (c NamespaceCheck) GetEndpoint() string {
	return c.CheckName
}

func (c NamespaceCheck) String() string {
	return "namespace/" + c.CheckName
}

func (c NamespaceCheck) GetType() string {
	return "namespace"
}

type DNSCheck struct {
	Description     `yaml:",inline" json:",inline"`
	Server          string   `yaml:"server" json:"server,omitempty"`
	Port            int      `yaml:"port,omitempty" json:"port,omitempty"`
	Query           string   `yaml:"query,omitempty" json:"query,omitempty"`
	QueryType       string   `yaml:"querytype,omitempty" json:"querytype,omitempty"`
	MinRecords      int      `yaml:"minrecords,omitempty" json:"minrecords,omitempty"`
	ExactReply      []string `yaml:"exactreply,omitempty" json:"exactreply,omitempty"`
	Timeout         int      `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ThresholdMillis int      `yaml:"thresholdMillis,omitempty" json:"thresholdMillis,omitempty"`
	// SrvReply    SrvReply `yaml:"srvReply,omitempty" json:"srvReply,omitempty"`
}

func (c DNSCheck) GetEndpoint() string {
	s := fmt.Sprintf("%s/%s", c.QueryType, c.Query)
	if c.Server != "" {
		s += "@" + c.Server
		if c.Port != 0 {
			s += fmt.Sprintf(":%d", c.Port)
		}
	}
	return s
}

func (c DNSCheck) GetType() string {
	return "dns"
}

type HelmCheck struct {
	Description `yaml:",inline" json:",inline"`
	Chartmuseum string          `yaml:"chartmuseum" json:"chartmuseum,omitempty"`
	Project     string          `yaml:"project,omitempty" json:"project,omitempty"`
	Auth        *Authentication `yaml:"auth,omitempty" json:"auth,omitempty"`
	CaFile      string          `yaml:"cafile,omitempty" json:"cafile,omitempty"`
}

func (c HelmCheck) GetEndpoint() string {
	return fmt.Sprintf("%s/%s", c.Chartmuseum, c.Project)
}

func (c HelmCheck) GetType() string {
	return "helm"
}

type JunitCheck struct {
	Description `yaml:",inline" json:",inline"`
	TestResults string `yaml:"testResults" json:"testResults"`
	Templatable `yaml:",inline" json:",inline"`
	// Timeout in minutes to wait for specified container to finish its job. Defaults to 5 minutes
	Timeout int        `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Spec    v1.PodSpec `yaml:"spec" json:"spec"`
}

func (c JunitCheck) GetEndpoint() string {
	if c.Description.String() != "" {
		return c.Description.String()
	}
	if len(c.Spec.Containers) > 0 {
		if c.Spec.Containers[0].Name != "" {
			return c.Spec.Containers[0].Name
		}
		if c.Spec.Containers[0].Image != "" {
			return c.Spec.Containers[0].Image
		}
	}
	return c.TestResults
}

func (c JunitCheck) GetTimeout() int {
	if c.Timeout != 0 {
		return c.Timeout
	}
	return 5
}

func (c JunitCheck) GetType() string {
	return "junit"
}

type SMBConnection struct {
	//Port on which smb server is running. Defaults to 445
	Port int             `yaml:"port,omitempty" json:"port,omitempty"`
	Auth *Authentication `yaml:"auth" json:"auth"`
	//Domain...
	Domain string `yaml:"domain,omitempty" json:"domain,omitempty"`
	// Workstation...
	Workstation string `yaml:"workstation,omitempty" json:"workstation,omitempty"`
	//Sharename to mount from the samba server
	Sharename string `yaml:"sharename,omitempty" json:"sharename,omitempty"`
	//SearchPath sub-path inside the mount location
	SearchPath string `yaml:"searchPath,omitempty" json:"searchPath,omitempty" `
}

func (c SMBConnection) GetPort() int {
	if c.Port != 0 {
		return c.Port
	}
	return 445
}

/*
[include:datasources/prometheus.yaml]

*/
type Prometheus struct {
	PrometheusCheck `yaml:",inline" json:",inline"`
}
type PrometheusCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	// Address of the prometheus server
	Host string `yaml:"host" json:"host" template:"true" `
	// PromQL query
	Query string `yaml:"query" json:"query" template:"true"`
}

func (c PrometheusCheck) GetType() string {
	return "prometheus"
}

func (c PrometheusCheck) GetEndpoint() string {
	return fmt.Sprintf("%v/%v", c.Host, c.Description)
}

type MongoDBCheck struct {
	Description `yaml:",inline" json:",inline"`
	// Monogodb connection string, e.g.  mongodb://:27017/?authSource=admin, See https://docs.mongodb.com/manual/reference/connection-string/
	Connection `yaml:",inline" json:",inline"`
}

func (c MongoDBCheck) GetType() string {
	return "mongodb"
}

// Git executes a SQL style query against a github repo using https://github.com/askgitdev/askgit
type Git struct {
	GitHubCheck `yaml:",inline" json:",inline"`
}

type GitHubCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	// Query to be executed. Please see https://github.com/askgitdev/askgit for more details regarding syntax
	Query       string          `yaml:"query" json:"query"`
	GithubToken *kommons.EnvVar `yaml:"githubToken,omitempty" json:"githubToken,omitempty"`
}

func (c GitHubCheck) GetType() string {
	return "github"
}

func (c GitHubCheck) GetEndpoint() string {
	return strings.ReplaceAll(c.Query, " ", "-")
}

type ResourceSelector struct {
	Name          string `yaml:"name,omitempty" json:"name,omitempty"`
	LabelSelector string `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	FieldSelector string `json:"fieldSelector,omitempty" yaml:"fieldSelector,omitempty"`
}

type KubernetesCreatorCheck struct {
	Description  `yaml:",inline" json:",inline"`
	Templatable  `yaml:",inline" json:",inline"`
	ResourceSpec []unstructured.Unstructured `yaml:"resourceSpec,omitempty" json:"resourceSpec,omitempty"`
	CanaryRef    []v1.LocalObjectReference   `yaml:"canaryRef,omitempty" json:"canaryRef,omitempty"`
}

func (c KubernetesCreatorCheck) GetEndpoint() string {
	if len(c.ResourceSpec) > 0 {
		return fmt.Sprintf("%v/%v", c.ResourceSpec[0].GetKind(), c.ResourceSpec[0].GetName())
	}
	return ""
}

func (c KubernetesCreatorCheck) GetType() string {
	return "KubernetesCreator"
}

type KubernetesCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Namespace   ResourceSelector `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Resource    ResourceSelector `yaml:"resource,omitempty" json:"resource,omitempty"`
	Kind        string           `yaml:"kind" json:"kind"`
	Ready       *bool            `yaml:"ready,omitempty" json:"ready,omitempty"`
}

func (c KubernetesCheck) GetType() string {
	return "kubernetes"
}

func (c KubernetesCheck) GetEndpoint() string {
	return fmt.Sprintf("%v/%v/%v", c.Kind, c.Description.Description, c.Namespace.Name)
}

func (c KubernetesCheck) CheckReady() bool {
	if c.Ready == nil {
		return true
	}
	return *c.Ready
}

type AWSConnection struct {
	AccessKey kommons.EnvVar `yaml:"accessKey" json:"accessKey"`
	SecretKey kommons.EnvVar `yaml:"secretKey" json:"secretKey"`
	Region    string         `yaml:"region,omitempty" json:"region"`
	Endpoint  string         `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	// Skip TLS verify when connecting to aws
	SkipTLSVerify bool `yaml:"skipTLSVerify,omitempty" json:"skipTLSVerify,omitempty"`
	// glob path to restrict matches to a subset
	ObjectPath string `yaml:"objectPath,omitempty" json:"objectPath,omitempty"`
	// Use path style path: http://s3.amazonaws.com/BUCKET/KEY instead of http://BUCKET.s3.amazonaws.com/KEY
	UsePathStyle bool `yaml:"usePathStyle,omitempty" json:"usePathStyle,omitempty"`
}

type GCPConnection struct {
	Endpoint    string          `yaml:"endpoint" json:"endpoint,omitempty"`
	Credentials *kommons.EnvVar `yaml:"credentials" json:"credentials,omitempty"`
}

type FolderCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	// Path can be the s3 or gcs bucket, or reference to local folder
	// for s3 the prefix need to be s3://<bucket-name> for gCloud, gcs://<bucket-name>
	Path           string       `yaml:"path" json:"path"`
	Filter         FolderFilter `yaml:"filter,omitempty" json:"filter,omitempty"`
	FolderTest     `yaml:",inline" json:",inline"`
	*AWSConnection `yaml:"awsConnection,omitempty" json:"awsConnection,omitempty"`
	*GCPConnection `yaml:"gcpConnection,omitempty" json:"gcpConnection,omitempty"`
	*SMBConnection `yaml:"smbConnection,omitempty" json:"smbConnection,omitempty"`
}

func (c FolderCheck) GetType() string {
	return "folder"
}

func (c FolderCheck) GetEndpoint() string {
	return c.Path
}

type ExecCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	// Script can be a inline script or a path to a script that needs to be executed
	// On windows executed via powershell and in darwin and linux executed using bash
	Script *string `yaml:"script" json:"script"`
}

func (c ExecCheck) GetType() string {
	return "exec"
}

func (c ExecCheck) GetEndpoint() string {
	return *c.Script
}

func (c ExecCheck) GetTestFunction() Template {
	if c.Test.Expression == "" {
		c.Test.Expression = "results.ExitCode == 0"
	}
	return c.Test
}

/*
[include:minimal/http_pass.yaml]
*/
type HTTP struct {
	HTTPCheck `yaml:",inline" json:"inline"`
}

/*

[include:minimal/dns_pass.yaml]

*/
type DNS struct {
	DNSCheck `yaml:",inline" json:"inline"`
}

/*
[include:k8s/docker_pass.yaml]
*/
type DockerPull struct {
	DockerPullCheck `yaml:",inline" json:"inline"`
}

/*
DockerPush check will try to push a Docker image to specified registry.
/*
[include:k8s/docker_push_pass.yaml]
*/
type DockerPush struct {
	DockerPushCheck `yaml:",inline" json:"inline"`
}

/*
S3 check will:

* list objects in the bucket to check for Read permissions
* PUT an object into the bucket for Write permissions
* download previous uploaded object to check for Get permissions

[include:aws/s3_bucket_pass.yaml]
*/
type S3 struct {
	S3Check `yaml:",inline" json:"inline"`
}

type TCP struct {
	TCPCheck `yaml:",inline" json:"inline"`
}

/*
[include:k8s/pod_pass.yaml]
*/
type Pod struct {
	PodCheck `yaml:",inline" json:"inline"`
}

/*

The LDAP check will:

* bind using provided user/password to the ldap host. Supports ldap/ldaps protocols.
* search an object type in the provided bind DN.s

[include:datasources/ldap_pass.yaml]

*/
type LDAP struct {
	LDAPCheck `yaml:",inline" json:"inline"`
}

/*

The Namespace check will:

* create a new namespace using the labels/annotations provided

[include:k8s/namespace_pass.yaml]

*/
type Namespace struct {
	NamespaceCheck `yaml:",inline" json:"inline"`
}

/*
This test will check ICMP packet loss and duration.

[include:quarantine/icmp_pass.yaml]

*/
type ICMP struct {
	ICMPCheck `yaml:",inline" json:"inline"`
}

/*
This check will try to connect to a specified Postgresql database, run a query against it and verify the results.

[include:datasources/postgres_pass.yaml]
*/
type Postgres struct {
	PostgresCheck `yaml:",inline" json:"inline"`
}

/*
This check will try to connect to a specified MsSQL database, run a query against it and verify the results.

[include:datasources/mssql_pass.yaml]

*/
type MsSQL struct {
	MssqlCheck `yaml:",inline" json:"inline"`
}

/*

[include:datasources/helm_pass.yaml]

*/
type Helm struct {
	HelmCheck `yaml:",inline" json:"inline"`
}

type SrvReply struct {
	Target   string `yaml:"target,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
	Weight   int    `yaml:"wight,omitempty"`
}

/*
This check will try to connect to a specified Redis instance, run a ping against it and verify the pong response.

[include:datasources/redis_pass.yaml]

*/

type Redis struct {
	RedisCheck `yaml:",inline" json:"inline"`
}

/*

This check will connect to a restic repository and perform Integrity and backup Freshness Tests

[include:datasources/restic_pass.yaml]

*/

type Restic struct {
	ResticCheck `yaml:",inline" json:"inline"`
}

/*
Jmeter check will run jmeter cli against the supplied host
[include:k8s/jmeter_pass.yaml]
*/
type Jmeter struct {
	JmeterCheck `yaml:",inline" json:",inline"`
}

/*
Junit check will wait for the given pod to be completed than parses all the xml files present in the defined testResults directory

[include:k8s/junit_pass.yaml]
*/
type Junit struct {
	JunitCheck `yaml:",inline" json:",inline"`
}

/*
This checks the cloudwatch for all the Active alarm and response with the reason
[include:aws/cloudwatch_pass.yaml]
*/
type CloudWatch struct {
	CloudWatchCheck `yaml:",inline" json:",inline"`
}

/*
[include:k8s/containerd_pull_pass.yaml]
*/
type ContainerdPull struct {
	ContainerdPullCheck `yaml:",inline" json:",inline"`
}

/*
[include:k8s/containerd_push_pass.yaml]
*/
type ContainerdPush struct {
	ContainerdPushCheck `yaml:",inline" json:",inline"`
}

/*
[include:k8s/kuberenetes_pass.yaml]
*/
type Kubernetes struct {
	KubernetesCheck `yaml:",inline" json:",inline"`
}

/*
Folder Check provides an abstraction over checker related to folder.
Currently, used to perform the following checks:
 - s3bucket check
	- search objects matching the provided object path pattern
	- check that latest object is no older than provided MaxAge value in seconds
	- check that latest object size is not smaller than provided MinSize value in bytes.
 - gcsBucket check
    - search objects matching the provided object path pattern
	- check that latest object is no older than provided MaxAge value in seconds
	- check that latest object size is not smaller than provided MinSize value in bytes.
 - Smb check: which connects to the given samba server with given credentials
	find the age of the latest updated file and compare it with minAge
	count the number of file present and compare with minCount if defined
 - local Folder check

[include:quarantine/smb_pass.yaml]
[include:datasources/s3_bucket_pass.yaml]
[include:datasource/folder_pass.yaml]
*/
type Folder struct {
	FolderCheck `yaml:",inline" json:",inline"`
}

/*
Exec Check executes a command or scrtipt file on the target host.
On Linux/MacOS uses bash and on Windows uses powershell.
[include:minimal/exec_pass.yaml]
*/
type Exec struct {
	ExecCheck `yaml:",inline" json:",inline"`
}

/*
[include:aws/ec2_pass.yaml]
*/
type EC2 struct {
	EC2Check `yaml:",inline" json:",inline"`
}
type EC2Check struct {
	Description   `yaml:",inline" json:",inline"`
	AWSConnection `yaml:",inline" json:",inline"`
	AMI           string                    `yaml:"ami,omitempty" json:"ami,omitempty"`
	UserData      string                    `yaml:"userData,omitempty" json:"userData,omitempty"`
	SecurityGroup string                    `yaml:"securityGroup,omitempty" json:"securityGroup,omitempty"`
	KeepAlive     bool                      `yaml:"keepAlive,omitempty" json:"keepAlive,omitempty"`
	WaitTime      int                       `yaml:"waitTime,omitempty" json:"waitTime,omitempty"`
	TimeOut       int                       `yaml:"timeOut,omitempty" json:"timeOut,omitempty"`
	CanaryRef     []v1.LocalObjectReference `yaml:"canaryRef,omitempty" json:"canaryRef,omitempty"`
}

func (c EC2Check) GetEndpoint() string {
	return c.Region
}

func (c EC2Check) GetType() string {
	return "ec2"
}

var AllChecks = []external.Check{
	HTTPCheck{},
	TCPCheck{},
	ICMPCheck{},
	S3Check{},
	DockerPullCheck{},
	DockerPushCheck{},
	ContainerdPullCheck{},
	ContainerdPushCheck{},
	PostgresCheck{},
	MssqlCheck{},
	RedisCheck{},
	PodCheck{},
	LDAPCheck{},
	ResticCheck{},
	NamespaceCheck{},
	DNSCheck{},
	HelmCheck{},
	JmeterCheck{},
	JunitCheck{},
	EC2Check{},
	PrometheusCheck{},
	MongoDBCheck{},
	CloudWatchCheck{},
	GitHubCheck{},
	Kubernetes{},
	FolderCheck{},
	ExecCheck{},
}
