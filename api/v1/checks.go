package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/shell"
	"github.com/flanksource/duty/types"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

// List of additional check label keys that should be included in the check metrics.
// By default the labels metrics are not exposed.
var AdditionalCheckMetricLabels []string

const (
	OnTransformMarkHealthy   = "MarkHealthy"
	OnTransformMarkUnhealthy = "MarkUnhealthy"
	OnTransformIgnore        = "Ignore"
)

type checkContext interface {
	context.Context
	HydrateConnectionByURL(connectionName string) (*models.Connection, error)
	GetEnvValueFromCache(env types.EnvVar, namespace string) (string, error)
}

type Check struct {
	Name, Type, Endpoint, Description, Icon string
	Labels                                  map[string]string
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

func (c Check) GetLabels() map[string]string {
	return c.Labels
}

type Artifact struct {
	// Path to the artifact on the check runner.
	// Special paths: /dev/stdout & /dev/stdin
	Path string `yaml:"path" json:"path"`
}

// type TestResult struct {
// 	Path string
// }

type Oauth2Config struct {
	Scopes   []string          `json:"scope,omitempty" yaml:"scope,omitempty"`
	TokenURL string            `json:"tokenURL,omitempty" yaml:"tokenURL,omitempty"`
	Params   map[string]string `json:"params,omitempty" yaml:"params,omitempty"`
}

type TLSConfig struct {
	// InsecureSkipVerify controls whether a client verifies the server's
	// certificate chain and host name
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	// HandshakeTimeout defaults to 10 seconds
	HandshakeTimeout Duration `json:"handshakeTimeout,omitempty" yaml:"handshakeTimeout,omitempty"`
	// PEM encoded certificate of the CA to verify the server certificate
	CA types.EnvVar `json:"ca,omitempty" yaml:"ca,omitempty"`
	// PEM encoded client certificate
	Cert types.EnvVar `json:"cert,omitempty" yaml:"cert,omitempty"`
	// PEM encoded client private key
	Key types.EnvVar `json:"key,omitempty" yaml:"key,omitempty"`
}

type Crawl struct {
	// Filters is a list of regex filters to apply to the crawled links.
	Filters []string `yaml:"filters,omitempty" json:"filters,omitempty"`
	// Depth is the maximum number of links to follow.
	Depth                int      `yaml:"depth,omitempty" json:"depth,omitempty"`
	AllowedDomains       []string `yaml:"allowedDomains,omitempty" json:"allowedDomains,omitempty"`
	DisallowedDomains    []string `yaml:"disallowedDomains,omitempty" json:"disallowedDomains,omitempty"`
	AllowedURLFilters    []string `yaml:"allowedURLFilters,omitempty" json:"allowedURLFilters,omitempty"`
	DisallowedURLFilters []string `yaml:"disallowedURLFilters,omitempty" json:"disallowedURLFilters,omitempty"`

	// Delay is the duration to wait before creating a new request to the matching domains, defaults to 500ms
	Delay Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	// RandomDelay is the extra randomized duration to wait added to Delay before creating a new request, defaults to 100ms
	RandomDelay Duration `yaml:"randomDelay,omitempty" json:"randomDelay,omitempty"`
	// Parallelism is the number of the maximum allowed concurrent requests, defaults to 2
	Parallelism int `yaml:"parallelism,omitempty" json:"parallelism,omitempty"`
}
type HTTPCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
	// Deprecated: Use url instead
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty" template:"true"`
	// Maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer.
	ThresholdMillis int `yaml:"thresholdMillis,omitempty" json:"thresholdMillis,omitempty"`
	// Expected response codes for the HTTP Request.
	ResponseCodes []int `yaml:"responseCodes,omitempty" json:"responseCodes,omitempty"`
	// Exact response content expected to be returned by the endpoint.
	ResponseContent string `yaml:"responseContent,omitempty" json:"responseContent,omitempty"`
	// Deprecated, use expr and jsonpath function
	ResponseJSONContent *JSONCheck `yaml:"responseJSONContent,omitempty" json:"responseJSONContent,omitempty"`
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
	Headers []types.EnvVar `yaml:"headers,omitempty" json:"headers,omitempty"`
	// Template the request body
	TemplateBody bool `yaml:"templateBody,omitempty" json:"templateBody,omitempty"`
	// EnvVars are the environment variables that are accessible to templated body
	EnvVars []types.EnvVar `yaml:"env,omitempty" json:"env,omitempty"`
	// Oauth2 Configuration. The client ID & Client secret should go to username & password respectively.
	Oauth2 *Oauth2Config `yaml:"oauth2,omitempty" json:"oauth2,omitempty"`
	// TLS Config
	TLSConfig *TLSConfig `yaml:"tlsConfig,omitempty" json:"tlsConfig,omitempty"`
	// Crawl site and verify links
	Crawl *Crawl `yaml:"crawl,omitempty" json:"crawl,omitempty"`
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
	Relatable       `yaml:",inline" json:",inline"`
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
	Relatable           `yaml:",inline" json:",inline"`
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
	Description             `yaml:",inline" json:",inline"`
	Relatable               `yaml:",inline" json:",inline"`
	connection.S3Connection `yaml:",inline" json:",inline"`
	BucketName              string `yaml:"bucketName" json:"bucketName,omitempty"`
	StorageClass            string `yaml:"storageClass" json:"storageClass,omitempty"`
}

func (c S3Check) GetEndpoint() string {
	return fmt.Sprintf("%s/%s", c.AWSConnection.Endpoint, c.BucketName)
}

func (c S3Check) GetType() string {
	return "s3"
}

type CloudWatchCheck struct {
	Description              `yaml:",inline" json:",inline"`
	connection.AWSConnection `yaml:",inline" json:",inline"`
	Templatable              `yaml:",inline" json:",inline"`
	Relatable                `yaml:",inline" json:",inline"`
	CloudWatchFilter         `yaml:",inline" json:",inline"`
}

type CloudWatchFilter struct {
	ActionPrefix *string  `yaml:"actionPrefix,omitempty" json:"actionPrefix,omitempty"`
	AlarmPrefix  *string  `yaml:"alarmPrefix,omitempty" json:"alarmPrefix,omitempty"`
	Alarms       []string `yaml:"alarms,omitempty" json:"alarms,omitempty"`
	State        string   `yaml:"state,omitempty" json:"state,omitempty"`
}

func (c CloudWatchCheck) GetEndpoint() string {
	endpoint := c.Region
	if c.CloudWatchFilter.ActionPrefix != nil {
		endpoint += "-" + *c.CloudWatchFilter.ActionPrefix
	}
	if c.CloudWatchFilter.AlarmPrefix != nil {
		endpoint += "-" + *c.CloudWatchFilter.AlarmPrefix
	}
	return endpoint
}

func (c CloudWatchCheck) GetType() string {
	return "cloudwatch"
}

type ResticCheck struct {
	Description `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// Name of the connection used to derive restic password.
	ConnectionName string `yaml:"connection,omitempty" json:"connection,omitempty"`
	// Name of the AWS connection used to derive the access key and secret key.
	AWSConnectionName string `yaml:"awsConnectionName,omitempty" json:"awsConnectionName,omitempty"`
	// Repository The restic repository path eg: rest:https://user:pass@host:8000/ or rest:https://host:8000/ or s3:s3.amazonaws.com/bucket_name
	Repository string `yaml:"repository" json:"repository"`
	// Password for the restic repository
	Password *types.EnvVar `yaml:"password" json:"password"`
	// MaxAge for backup freshness
	MaxAge string `yaml:"maxAge" json:"maxAge"`
	// CheckIntegrity when enabled will check the Integrity and consistency of the restic reposiotry
	CheckIntegrity bool `yaml:"checkIntegrity,omitempty" json:"checkIntegrity,omitempty"`
	// AccessKey access key id for connection with aws s3, minio, wasabi, alibaba oss
	AccessKey *types.EnvVar `yaml:"accessKey,omitempty" json:"accessKey,omitempty"`
	// SecretKey secret access key for connection with aws s3, minio, wasabi, alibaba oss
	SecretKey *types.EnvVar `yaml:"secretKey,omitempty" json:"secretKey,omitempty"`
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
	Relatable   `yaml:",inline" json:",inline"`
	// Jmx defines the ConfigMap or Secret reference to get the JMX test plan
	Jmx types.EnvVar `yaml:"jmx" json:"jmx"`
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
	Relatable      `yaml:",inline" json:",inline"`
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
	Relatable   `yaml:",inline" json:",inline"`
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
	Relatable      `yaml:",inline" json:",inline"`
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
	Relatable   `yaml:",inline" json:",inline"`
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
	Relatable   `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
	// Deprecated: Use url instead
	Addr string `yaml:"addr,omitempty" json:"addr,omitempty" template:"true"`
	DB   *int   `yaml:"db,omitempty" json:"db,omitempty"`
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
	Relatable   `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
	Timeout     int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
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

func (c *SQLCheck) GetQueryTimeout() time.Duration {
	if c.Timeout == 0 {
		c.Timeout = 60
	}

	return time.Duration(c.Timeout) * time.Second
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

func (p PostgresCheck) GetCheck() external.Check {
	return p
}

func (p PostgresCheck) GetType() string {
	return "postgres" //nolint
}

func (p PostgresCheck) GetDriver() string {
	return "postgres"
}

type MssqlCheck struct {
	SQLCheck `yaml:",inline" json:",inline"`
}

func (m MssqlCheck) GetCheck() external.Check {
	return m
}

func (m MssqlCheck) GetSQLCheck() SQLCheck {
	return m.SQLCheck
}

func (m MysqlCheck) GetSQLCheck() SQLCheck {
	return m.SQLCheck
}

func (p PostgresCheck) GetSQLCheck() SQLCheck {
	return p.SQLCheck
}

func (m MssqlCheck) GetDriver() string {
	return "mssql"
}

func (m MssqlCheck) GetType() string {
	return "mssql"
}

type MysqlCheck struct {
	SQLCheck `yaml:",inline" json:",inline"`
}

func (m MysqlCheck) GetCheck() external.Check {
	return m
}

func (m MysqlCheck) GetDriver() string {
	return "mysql"
}

func (m MysqlCheck) GetType() string {
	return "mysql"
}

/*
[include:datasources/mongo_pass.yaml]
*/
type Mongo struct {
	MongoDBCheck `yaml:",inline" json:",inline"`
}

type OpenSearchCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
	Query       string `yaml:"query" json:"query"`
	Index       string `yaml:"index" json:"index"`
	Results     int64  `yaml:"results,omitempty" json:"results,omitempty"`
}

func (c OpenSearchCheck) GetType() string {
	return "opensearch"
}

func (c OpenSearchCheck) GetEndpoint() string {
	return c.URL
}

/*
[include:datasources/elasticsearch_pass.yaml]
*/

type Elasticsearch struct {
	ElasticsearchCheck `yaml:",inline" json:",inline"`
}

type ElasticsearchCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
	Query       string `yaml:"query" json:"query,omitempty" template:"true"`
	Index       string `yaml:"index" json:"index,omitempty" template:"true"`
	Results     int    `yaml:"results" json:"results,omitempty" template:"true"`
}

func (c ElasticsearchCheck) GetType() string {
	return "elasticsearch"
}

type DynatraceCheck struct {
	Description    `yaml:",inline" json:",inline"`
	Templatable    `yaml:",inline" json:",inline"`
	Relatable      `yaml:",inline" json:",inline"`
	ConnectionName string       `yaml:"connection,omitempty" json:"connection,omitempty"`
	Host           string       `yaml:"host" json:"host,omitempty" template:"true"`
	Scheme         string       `yaml:"scheme" json:"scheme,omitempty"`
	APIKey         types.EnvVar `yaml:"apiKey" json:"apiKey,omitempty"`
	Namespace      string       `yaml:"namespace" json:"namespace,omitempty" template:"true"`
}

func (t DynatraceCheck) GetType() string {
	return "dynatrace"
}

func (t DynatraceCheck) GetEndpoint() string {
	return fmt.Sprintf("%s://%s", t.Scheme, t.Host)
}

/*
[include:datasources/alertmanager_mix.yaml]
*/

type AlertManager struct {
	AlertManagerCheck `yaml:",inline" json:",inline"`
}

// CheckRelationship defines a way to link the check results to components and configs
// using lookup expressions.
type CheckRelationship struct {
	Components []duty.RelationshipSelectorTemplate `yaml:"components,omitempty" json:"components,omitempty"`
	Configs    []duty.RelationshipSelectorTemplate `yaml:"configs,omitempty" json:"configs,omitempty"`
}

type Relatable struct {
	// Relationships defines a way to link the check results to components and configs
	// using lookup expressions.
	Relationships *CheckRelationship `yaml:"relationships,omitempty" json:"relationships,omitempty"`
}

func (t Relatable) GetRelationship() *CheckRelationship {
	return t.Relationships
}

type AlertManagerCheck struct {
	Description    `yaml:",inline" json:",inline"`
	Templatable    `yaml:",inline" json:",inline"`
	Connection     `yaml:",inline" json:",inline"`
	Relatable      `yaml:",inline" json:",inline"`
	Alerts         []string          `yaml:"alerts" json:"alerts,omitempty" template:"true"`
	Filters        map[string]string `yaml:"filters" json:"filters,omitempty" template:"true"`
	ExcludeFilters map[string]string `yaml:"exclude_filters" json:"exclude_filters,omitempty" template:"true"`
	Ignore         []string          `yaml:"ignore" json:"ignore,omitempty" template:"true"`
}

func (c AlertManagerCheck) GetType() string {
	return "alertmanager"
}

type PodCheck struct {
	Description          `yaml:",inline" json:",inline"`
	Relatable            `yaml:",inline" json:",inline"`
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
	IngressClass         string `yaml:"ingressClass" json:"ingressClass,omitempty"`
	ExpectedContent      string `yaml:"expectedContent,omitempty" json:"expectedContent,omitempty" template:"true"`
	ExpectedHTTPStatuses []int  `yaml:"expectedHttpStatuses,omitempty" json:"expectedHttpStatuses,omitempty"`
	PriorityClass        string `yaml:"priorityClass,omitempty" json:"priorityClass,omitempty"`
	RoundRobinNodes      bool   `yaml:"roundRobinNodes,omitempty" json:"roundRobinNodes,omitempty"`
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
	Relatable     `yaml:",inline" json:",inline"`
	Connection    `yaml:",inline" json:",inline"`
	BindDN        string `yaml:"bindDN" json:"bindDN"`
	UserSearch    string `yaml:"userSearch,omitempty" json:"userSearch,omitempty"`
	SkipTLSVerify bool   `yaml:"skipTLSVerify,omitempty" json:"skipTLSVerify,omitempty"`
}

func (c LDAPCheck) GetType() string {
	return "ldap"
}

type NamespaceCheck struct {
	Description          `yaml:",inline" json:",inline"`
	Relatable            `yaml:",inline" json:",inline"`
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
	return c.Name
}

func (c NamespaceCheck) String() string {
	return "namespace/" + c.Name
}

func (c NamespaceCheck) GetType() string {
	return "namespace"
}

type DNSCheck struct {
	Description     `yaml:",inline" json:",inline"`
	Relatable       `yaml:",inline" json:",inline"`
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
	Relatable   `yaml:",inline" json:",inline"`
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
	Relatable   `yaml:",inline" json:",inline"`
	// Timeout in minutes to wait for specified container to finish its job. Defaults to 5 minutes
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	Spec json.RawMessage `yaml:"spec" json:"spec"`
	// Artifacts configure the artifacts generated by the check
	Artifacts []Artifact `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
}

func (c JunitCheck) GetEndpoint() string {
	if c.Description.String() != "" {
		return c.Description.String()
	}
	// if len(c.Spec.Containers) > 0 {
	// 	if c.Spec.Containers[0].Name != "" {
	// 		return c.Spec.Containers[0].Name
	// 	}
	// 	if c.Spec.Containers[0].Image != "" {
	// 		return c.Spec.Containers[0].Image
	// 	}
	// }
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

/*
[include:datasources/prometheus.yaml]
*/
type Prometheus struct {
	PrometheusCheck `yaml:",inline" json:",inline"`
}

type PrometheusCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// Deprecated: use `url` instead
	Host                      string `yaml:"host,omitempty" json:"host,omitempty"`
	connection.HTTPConnection `yaml:",inline" json:",inline"`
	// PromQL query
	Query string `yaml:"query" json:"query" template:"true"`
}

func (c PrometheusCheck) GetType() string {
	return "prometheus"
}

type MongoDBCheck struct {
	Description `yaml:",inline" json:",inline"`
	Connection  `yaml:",inline" json:",inline"`
}

func (c MongoDBCheck) GetType() string {
	return "mongodb"
}

// Git executes a SQL style query against a github repo using https://github.com/askgitdev/askgit
type Git struct {
	GitHubCheck `yaml:",inline" json:",inline"`
}

type GitHubCheck struct {
	Description    `yaml:",inline" json:",inline"`
	Templatable    `yaml:",inline" json:",inline"`
	Relatable      `yaml:",inline" json:",inline"`
	ConnectionName string `yaml:"connection,omitempty" json:"connection,omitempty"`
	// Query to be executed. Please see https://github.com/askgitdev/askgit for more details regarding syntax
	Query       string       `yaml:"query" json:"query"`
	GithubToken types.EnvVar `yaml:"githubToken,omitempty" json:"githubToken,omitempty"`
}

func (c GitHubCheck) GetType() string {
	return "github"
}

func (c GitHubCheck) GetEndpoint() string {
	return strings.ReplaceAll(c.Query, " ", "-")
}

type GitProtocolCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	FileName    string       `yaml:"filename,omitempty" json:"filename,omitempty"`
	Repository  string       `yaml:"repository" json:"repository"`
	Username    types.EnvVar `yaml:"username" json:"username"`
	Password    types.EnvVar `yaml:"password" json:"password"`
}

func (c GitProtocolCheck) GetType() string {
	return "gitProtocol"
}

func (c GitProtocolCheck) GetEndpoint() string {
	return strings.ReplaceAll(c.Repository, "/", "-")
}

type CatalogCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	Selector    types.ResourceSelectors `yaml:"selector" json:"selector"`
}

func (c CatalogCheck) GetType() string {
	return "catalog"
}

func (c CatalogCheck) GetEndpoint() string {
	return c.Selector.Hash()
}

// KubernetesResourceChecks is the canary spec.
// NOTE: It's only created to make crd generation possible.
// embedding CanarySpec into KubernetesResourceCheck.checks
// directly generates an invalid crd.
type KubernetesResourceChecks struct {
	CanarySpec `yaml:",inline" json:",inline"`
}

type KubernetesResourceCheckRetries struct {
	// Delay is the initial delay
	Delay    *Duration `json:"delay,omitempty"`
	Timeout  *Duration `json:"timeout,omitempty"`
	Interval *Duration `json:"interval,omitempty"`

	parsedDelay    *time.Duration `json:"-"`
	parsedTimeout  *time.Duration `json:"-"`
	parsedInterval *time.Duration `json:"-"`
}

func (t *KubernetesResourceCheckRetries) GetDelay() (time.Duration, error) {
	if t.Delay == nil {
		return time.Duration(0), nil
	}
	return t.Delay.GetDurationOrZero()
}

func (t *KubernetesResourceCheckRetries) GetTimeout() (time.Duration, error) {
	if t.Timeout == nil {
		return time.Duration(0), nil
	}
	return t.Timeout.GetDurationOrZero()
}

func (t *KubernetesResourceCheckRetries) GetInterval() (time.Duration, error) {
	if t.Interval == nil {
		return time.Duration(0), nil
	}
	return t.Interval.GetDurationOrZero()
}

type KubernetesResourceCheckWaitFor struct {
	// Expr is a cel expression that determines whether all the resources
	// are in their desired state before running checks on them.
	// 	Default: `dyn(resources).all(r, k8s.isHealthy(r))`
	Expr string `json:"expr,omitempty"`

	// Disable waiting for resources to get to their desired state.
	Disable bool `json:"disable,omitempty"`

	// Whether to wait for deletion or not
	Delete bool `json:"delete,omitempty"`

	// Timeout to wait for all static & non-static resources to be ready.
	// 	Default: 10m
	Timeout *Duration `json:"timeout,omitempty"`

	// Interval to check if all static & non-static resources are ready.
	// 	Default: 5s
	Interval *Duration `json:"interval,omitempty"`
}

type KubernetesResourceCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// StaticResources are kubernetes resources that are created & only
	// cleared when the canary is deleted
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	StaticResources []unstructured.Unstructured `json:"staticResources,omitempty"`

	// Resources are kubernetes resources that are created & cleared
	// after every check run.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Resources []unstructured.Unstructured `json:"resources"`

	// Checks to run against the kubernetes resources.
	// +kubebuilder:validation:XPreserveUnknownFields
	Checks []KubernetesResourceChecks `json:"checks,omitempty"`

	// Set initial delays and retry intervals for checks.
	CheckRetries KubernetesResourceCheckRetries `json:"checkRetries,omitempty"`

	// Ensure that the resources are deleted before creating them.
	ClearResources bool `json:"clearResources,omitempty"`

	connection.KubernetesConnection `json:",inline" yaml:",inline"`

	WaitFor KubernetesResourceCheckWaitFor `json:"waitFor,omitempty"`
}

func (c *KubernetesResourceCheck) HasResourcesWithMissingNamespace() bool {
	for _, r := range append(c.StaticResources, c.Resources...) {
		if r.GetNamespace() == "" {
			return true
		}
	}

	return false
}

// SetMissingNamespace will set the parent canaries name to resources whose namespace
// is not explicitly specified.
func (c *KubernetesResourceCheck) SetMissingNamespace(parent Canary, namespacedResources map[schema.GroupVersionKind]bool) {
	for i, r := range c.StaticResources {
		if r.GetNamespace() == "" && namespacedResources[r.GroupVersionKind()] {
			c.StaticResources[i].SetNamespace(parent.GetNamespace())
		}
	}

	for i, r := range c.Resources {
		if r.GetNamespace() == "" && namespacedResources[r.GroupVersionKind()] {
			c.Resources[i].SetNamespace(parent.GetNamespace())
		}
	}
}

func (c *KubernetesResourceCheck) SetCanaryOwnerReference(parent Canary) {
	var (
		id        = parent.GetPersistedID()
		name      = parent.GetName()
		namespace = parent.GetNamespace()
	)

	if id == "" || name == "" {
		// if the canary isn't persisted
		return
	}

	if namespace == "" {
		// we don't know the canaries namespace
		// so we can't set it in the owner references.
		return
	}

	canaryOwnerRef := metav1.OwnerReference{
		APIVersion: "canaries.flanksource.com/v1",
		Kind:       "Canary",
		Name:       name,
		UID:        k8sTypes.UID(id),
		Controller: lo.ToPtr(true),
	}

	for i, resource := range c.StaticResources {
		if resource.GetNamespace() != namespace {
			// the canary and the resource to be created are in different namespaces.
			// ownerRef enforces the owner to be in the same repo.
			continue
		}

		ownerRefs := resource.GetOwnerReferences()
		ownerRefs = append(ownerRefs, canaryOwnerRef)
		c.StaticResources[i].SetOwnerReferences(ownerRefs)
	}

	for i, resource := range c.Resources {
		if resource.GetNamespace() != namespace {
			continue
		}

		ownerRefs := resource.GetOwnerReferences()
		ownerRefs = append(ownerRefs, canaryOwnerRef)
		c.Resources[i].SetOwnerReferences(ownerRefs)
	}
}

func (c KubernetesResourceCheck) GetDisplayTemplate() Template {
	if !c.Templatable.Display.IsEmpty() {
		return c.Templatable.Display
	}

	return Template{
		Expression: "display.keys().map(k, k + ': ' + display[k]).join('\n')",
	}
}

func (c KubernetesResourceCheck) TotalResources() int {
	return len(c.Resources) + len(c.StaticResources)
}

func (c KubernetesResourceCheck) GetType() string {
	return "kubernetes_resource"
}

func (c KubernetesResourceCheck) GetEndpoint() string {
	return c.Name
}

type ResourceSelector struct {
	Name          string `yaml:"name,omitempty" json:"name,omitempty"`
	LabelSelector string `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	FieldSelector string `json:"fieldSelector,omitempty" yaml:"fieldSelector,omitempty"`
	Search        string `json:"search,omitempty" yaml:"search,omitempty"`
}

func (rs ResourceSelector) ToDutySelector() types.ResourceSelector {
	return types.ResourceSelector{
		Name:          rs.Name,
		LabelSelector: rs.LabelSelector,
		FieldSelector: rs.FieldSelector,
		Search:        rs.Search,
	}
}

type KubernetesCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	Namespace   ResourceSelector `yaml:"namespaceSelector,omitempty" json:"namespaceSelector,omitempty"`
	Resource    ResourceSelector `yaml:"resource,omitempty" json:"resource,omitempty"`
	// KubeConfig is the kubeconfig or the path to the kubeconfig file.
	connection.KubernetesConnection `yaml:",inline" json:",inline"`
	// Ignore the specified resources from the fetched resources. Can be a glob pattern.
	Ignore []string `yaml:"ignore,omitempty" json:"ignore,omitempty"`
	Kind   string   `yaml:"kind" json:"kind"`

	// Fail the check if any resources are unhealthy
	Healthy bool `yaml:"healthy,omitempty" json:"healthy,omitempty"`

	// Fail the check if any resources are not ready
	Ready bool `yaml:"ready,omitempty" json:"ready,omitempty"`
}

func (c KubernetesCheck) GetType() string {
	return "kubernetes"
}

func (c KubernetesCheck) GetEndpoint() string {
	return fmt.Sprintf("%v/%v/%v", c.Kind, c.Description.Description, c.Namespace.Name)
}

type AzureConnection struct {
	ConnectionName string        `yaml:"connection,omitempty" json:"connection,omitempty"`
	ClientID       *types.EnvVar `yaml:"clientID,omitempty" json:"clientID,omitempty"`
	ClientSecret   *types.EnvVar `yaml:"clientSecret,omitempty" json:"clientSecret,omitempty"`
	TenantID       string        `yaml:"tenantID,omitempty" json:"tenantID,omitempty"`
}

// HydrateConnection attempts to find the connection by name
// and populate the endpoint and credentials.
func (g *AzureConnection) HydrateConnection(ctx checkContext) error {
	connection, err := ctx.HydrateConnectionByURL(g.ConnectionName)
	if err != nil {
		return err
	}

	if connection != nil {
		g.ClientID = &types.EnvVar{ValueStatic: connection.Username}
		g.ClientSecret = &types.EnvVar{ValueStatic: connection.Password}
		g.TenantID = connection.Properties["tenantID"]
	}

	return nil
}

type FolderCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// Path  to folder or object storage, e.g. `s3://<bucket-name>`,  `gcs://<bucket-name>`, `/path/tp/folder`
	Path string `yaml:"path" json:"path"`
	// Recursive when set to true will recursively scan the folder to list the files in it.
	// However, symlinks are simply listed but not traversed.
	Recursive                  bool         `yaml:"recursive,omitempty" json:"recursive,omitempty"`
	Filter                     FolderFilter `yaml:"filter,omitempty" json:"filter,omitempty"`
	FolderTest                 `yaml:",inline" json:",inline"`
	*connection.S3Connection   `yaml:"awsConnection,omitempty" json:"awsConnection,omitempty"`
	*connection.GCSConnection  `yaml:"gcpConnection,omitempty" json:"gcpConnection,omitempty"`
	*connection.SMBConnection  `yaml:"smbConnection,omitempty" json:"smbConnection,omitempty"`
	*connection.SFTPConnection `yaml:"sftpConnection,omitempty" json:"sftpConnection,omitempty"`
}

func (c FolderCheck) GetType() string {
	return "folder"
}

func (c FolderCheck) GetEndpoint() string {
	return c.Path
}

type ExecConnections struct {
	AWS   *connection.AWSConnection `yaml:"aws,omitempty" json:"aws,omitempty"`
	GCP   *connection.GCPConnection `yaml:"gcp,omitempty" json:"gcp,omitempty"`
	Azure *AzureConnection          `yaml:"azure,omitempty" json:"azure,omitempty"`
}

type GitCheckout struct {
	URL         string       `yaml:"url,omitempty" json:"url,omitempty"`
	Connection  string       `yaml:"connection,omitempty" json:"connection,omitempty"`
	Username    types.EnvVar `yaml:"username,omitempty" json:"username,omitempty"`
	Password    types.EnvVar `yaml:"password,omitempty" json:"password,omitempty"`
	Certificate types.EnvVar `yaml:"certificate,omitempty" json:"certificate,omitempty"`
	// Destination is the full path to where the contents of the URL should be downloaded to.
	// If left empty, the sha256 hash of the URL will be used as the dir name.
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty"`
}

func (git GitCheckout) GetURL() types.EnvVar {
	return types.EnvVar{ValueStatic: git.URL}
}

func (git GitCheckout) GetUsername() types.EnvVar {
	return git.Username
}

func (git GitCheckout) GetPassword() types.EnvVar {
	return git.Password
}

func (git GitCheckout) GetCertificate() types.EnvVar {
	return git.Certificate
}

type ExecCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// Script can be a inline script or a path to a script that needs to be executed
	// On windows executed via powershell and in darwin and linux executed using bash
	Script      string                     `yaml:"script" json:"script"`
	Connections connection.ExecConnections `yaml:"connections,omitempty" json:"connections,omitempty"`
	// EnvVars are the environment variables that are accessible to exec processes
	EnvVars []types.EnvVar `yaml:"env,omitempty" json:"env,omitempty"`
	// Checkout details the git repository that should be mounted to the process
	Checkout *connection.GitConnection `yaml:"checkout,omitempty" json:"checkout,omitempty"`
	// Artifacts configure the artifacts generated by the check
	Artifacts []shell.Artifact `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
}

func (c ExecCheck) GetType() string {
	return "exec"
}

func (c ExecCheck) GetEndpoint() string {
	return c.Script
}

func (c ExecCheck) GetTestFunction() Template {
	if c.Test.Expression == "" {
		c.Test.Expression = "results.exitCode == 0"
	}
	return c.Test
}

type AwsConfigCheck struct {
	Description               `yaml:",inline" json:",inline"`
	Templatable               `yaml:",inline" json:",inline"`
	Relatable                 `yaml:",inline" json:",inline"`
	Query                     string `yaml:"query" json:"query"`
	*connection.AWSConnection `yaml:",inline" json:",inline"`
	AggregatorName            *string `yaml:"aggregatorName,omitempty" json:"aggregatorName,omitempty"`
}

func (c AwsConfigCheck) GetType() string {
	return "awsconfig"
}

func (c AwsConfigCheck) GetEndpoint() string {
	return c.Query
}

type AwsConfigRuleCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// List of rules which would be omitted from the fetch result
	IgnoreRules []string `yaml:"ignoreRules,omitempty" json:"ignoreRules,omitempty"`
	// Specify one or more Config rule names to filter the results by rule.
	Rules []string `yaml:"rules,omitempty" json:"rules,omitempty"`
	// Filters the results by compliance. The allowed values are INSUFFICIENT_DATA, NON_COMPLIANT, NOT_APPLICABLE, COMPLIANT
	ComplianceTypes           []string `yaml:"complianceTypes,omitempty" json:"complianceTypes,omitempty"`
	*connection.AWSConnection `yaml:",inline" json:",inline"`
}

func (c AwsConfigRuleCheck) GetType() string {
	return "awsconfigrule"
}

func (c AwsConfigRuleCheck) GetEndpoint() string {
	return c.Description.Description
}

type DatabaseBackupCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	GCP         *GCPDatabase `yaml:"gcp,omitempty" json:"gcp,omitempty"`
	MaxAge      Duration     `yaml:"maxAge,omitempty" json:"maxAge,omitempty"`
}

type GCPDatabase struct {
	Project                   string `yaml:"project" json:"project"`
	Instance                  string `yaml:"instance" json:"instance"`
	*connection.GCPConnection `yaml:"gcpConnection,omitempty" json:"gcpConnection,omitempty"`
}

func (c DatabaseBackupCheck) GetType() string {
	return "databasebackupcheck"
}

func (c DatabaseBackupCheck) GetEndpoint() string {
	return c.Description.Description
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
[include:k8s/kubernetes_pass.yaml]
*/
type Kubernetes struct {
	KubernetesCheck `yaml:",inline" json:",inline"`
}

/*
The folder check lists files in a folder (local or SMB/CIFS) or object storage platform like S3 or GCS and provides a mechanism to test:

* `minAge` - A file has been added within at least minAge e.g Has a backup been created in the last 24h
* `maxAge` - A file has been added and not removed within maxAge e.g. Has a file been processed in less than 24h
* `minSize` -
* `maxSize` -
* `minCount` -
* `maxCount` -

[include:quarantine/smb_pass.yaml]
[include:datasources/s3_bucket_pass.yaml]
[include:datasources/folder_pass.yaml]
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
AwsConfig check runs the given query against the AWS resources.
[include:aws/aws_config_pass.yaml]
*/
type AwsConfig struct {
	AwsConfigCheck `yaml:",inline" json:",inline"`
}

/*
[include:aws/aws_config_rule_pass.yaml]
*/
type AwsConfigRule struct {
	AwsConfigRuleCheck `yaml:",inline" json:",inline"`
}

/*
[include:datasources/database_backup.yaml]
*/
type DatabaseBackup struct {
	DatabaseBackupCheck `yaml:",inline" json:",inline"`
}

type AzureDevopsCheck struct {
	Description         `yaml:",inline" json:",inline"`
	Templatable         `yaml:",inline" json:",inline"`
	Relatable           `yaml:",inline" json:",inline"`
	ConnectionName      string            `yaml:"connection,omitempty" json:"connection,omitempty"`
	Organization        string            `yaml:"organization" json:"organization"`
	PersonalAccessToken types.EnvVar      `yaml:"personalAccessToken" json:"personalAccessToken"`
	Project             string            `yaml:"project" json:"project"` // Name or ID of the Project
	Pipeline            string            `yaml:"pipeline" json:"pipeline"`
	Variables           map[string]string `yaml:"variables" json:"variables"`
	Branches            []string          `ymal:"branch" json:"branch"`

	// ThresholdMillis the maximum duration of a Run. (Optional)
	ThresholdMillis *int `yaml:"thresholdMillis" json:"thresholdMillis"`
}

func (c AzureDevopsCheck) GetUsername() types.EnvVar {
	return types.EnvVar{ValueStatic: c.Organization}
}

func (c AzureDevopsCheck) GetPassword() types.EnvVar {
	return c.PersonalAccessToken
}

func (c AzureDevopsCheck) GetType() string {
	return "azuredevops"
}

func (c AzureDevopsCheck) GetEndpoint() string {
	return c.Project
}

type WebhookCheck struct {
	Description `yaml:",inline" json:",inline"`
	Templatable `yaml:",inline" json:",inline"`
	Relatable   `yaml:",inline" json:",inline"`
	// Token is an optional authorization token to run this check
	Token *types.EnvVar `yaml:"token,omitempty" json:"token,omitempty"`
}

func (c WebhookCheck) GetType() string {
	return "webhook"
}

func (c WebhookCheck) GetEndpoint() string {
	return ""
}

var AllChecks = []external.Check{
	AlertManagerCheck{},
	AwsConfigCheck{},
	AwsConfigRuleCheck{},
	AzureDevopsCheck{},
	CloudWatchCheck{},
	CatalogCheck{},
	ContainerdPullCheck{},
	ContainerdPushCheck{},
	DatabaseBackupCheck{},
	DNSCheck{},
	DockerPullCheck{},
	DockerPushCheck{},
	DynatraceCheck{},
	ElasticsearchCheck{},
	ExecCheck{},
	FolderCheck{},
	GitHubCheck{},
	GitProtocolCheck{},
	HelmCheck{},
	HTTPCheck{},
	ICMPCheck{},
	JmeterCheck{},
	JunitCheck{},
	Kubernetes{},
	LDAPCheck{},
	MongoDBCheck{},
	MssqlCheck{},
	MysqlCheck{},
	NamespaceCheck{},
	OpenSearchCheck{},
	PodCheck{},
	PostgresCheck{},
	PrometheusCheck{},
	RedisCheck{},
	ResticCheck{},
	S3Check{},
	TCPCheck{},
	WebhookCheck{},
}
