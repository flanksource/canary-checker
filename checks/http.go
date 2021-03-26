package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/flanksource/kommons"
	"github.com/pkg/errors"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/PaesslerAG/jsonpath"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
)

var (
	responseStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_http_response_status",
			Help: "The response status for HTTP checks per route.",
		},
		[]string{"status", "statusClass", "url"},
	)

	sslExpiration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_http_ssl_expiry",
			Help: "The number of days until ssl expiration",
		},
		[]string{"url"},
	)
)

func init() {
	prometheus.MustRegister(responseStatus, sslExpiration)
}

type HttpChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

// Type: returns checker type
func (c *HttpChecker) Type() string {
	return "http"
}

func (c *HttpChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c HttpChecker) GetClient() *kommons.Client {
	return c.kommons
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *HttpChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.HTTP {
		results = append(results, c.Check(conf))
	}
	return results
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *HttpChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.HTTPCheck)
	endpoint := check.Endpoint
	namespace := check.Namespace
	specNamespace := check.GetNamespace()



	if endpoint == "" && namespace == "" {
		return Failf(check, "One of Namespace or Endpoint must be specified")
	} else if endpoint != "" && namespace != "" {
		return Failf(check, "Namespace and Endpoint are mutually exclusive, only one may be specified")
	}
	if namespace == "*" {
		namespace = metav1.NamespaceAll
	}
	var lookupResult []pkg.URL
	if endpoint != "" {
		var err error
		lookupResult, err = DNSLookup(endpoint)
		if err != nil {
			return Failf(check, "failed to resolve DNS for %s", endpoint)
		}
	} else {
		k8sClient, err := pkg.NewK8sClient()
		if err != nil {
			return Failf(check, fmt.Sprintf("Unable to connect to k8s: %v", err))
		}
		serviceList, err := k8sClient.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return Failf(check, fmt.Sprintf("failed to obtain service list for namespace %v: %v", namespace, err))
		}
		for _, service := range serviceList.Items {
			endPoints, err := k8sClient.CoreV1().Endpoints(namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
			if err != nil {
				return Failf(check, fmt.Sprintf("Failed to obtain endpoints for service %v: %v", service.Name, err))
			}

			for _, endPoint := range endPoints.Subsets {
				for _, port := range service.Spec.Ports {
					if port.Port%1000 == 443 || port.TargetPort.IntVal%1000 == 443 {
						for _, address := range endPoint.Addresses {
							lookupResult = append(lookupResult, pkg.URL{
								IP:     address.IP,
								Port:   int(port.TargetPort.IntVal),
								Host:   address.Hostname,
								Scheme: "https",
							})
						}
					}
				}
			}
		}
	}

	username, password, err := c.ParseAuth(check)
	if err != nil {
		return Failf(check, "Failed to lookup authentication info %v:", err)
	}
	var headers map[string]string
	kommons := c.GetClient()
	for _, header := range check.Headers {
		if kommons == nil {
			return Failf(check, "Kommons client not set for HttpChecker instance")
		}
		key, value, err := kommons.GetEnvValue(header, specNamespace)
		if err != nil {
			return Failf(check, "Failed to parse header value: %v", err)
		}
		if headers == nil {
			headers = map[string]string{key: value}
		} else {
			headers[key] = value
		}
	}

	for _, urlObj := range lookupResult {

		if check.Method == "" {
			urlObj.Method = "GET"
		} else {
			urlObj.Method = check.Method
		}
		urlObj.Headers = headers
		urlObj.Body = check.Body
		urlObj.Username = username
		urlObj.Password = password
		checkResults, err := c.checkHTTP(urlObj)
		if err != nil {
			return invalidErrorf(check, err, "")
		}
		rcOK := false
		for _, rc := range check.ResponseCodes {
			if rc == checkResults.ResponseCode {
				rcOK = true
			}
		}

		if !rcOK {
			return Failf(check, "response code invalid %d != %v", checkResults.ResponseCode, check.ResponseCodes)
		}

		if check.ThresholdMillis > 0 && check.ThresholdMillis < int(checkResults.ResponseTime) {
			return Failf(check, "threshold exceeded %d > %d", checkResults.ResponseTime, check.ThresholdMillis)
		}
		if check.ResponseContent != "" && !strings.Contains(checkResults.Content, check.ResponseContent) {
			return Failf(check, "Expected %v, found %v", check.ResponseContent, checkResults.Content)
		}
		if check.ResponseJSONContent.Path != "" {
			var jsonContent interface{}
			if err = json.Unmarshal([]byte(checkResults.Content), &jsonContent); err != nil {
				return Failf(check, "Could not unmarshal response for json check: %v ", err)
			}

			jsonResult, err := jsonpath.Get(check.ResponseJSONContent.Path, jsonContent)
			if err != nil {
				return Failf(check, "Could not extract path %v from response %v: %v", check.ResponseJSONContent.Path, jsonContent, err)
			}
			switch s := jsonResult.(type) {
			case string:
				if s != check.ResponseJSONContent.Value {
					return Failf(check, "%v not equal to %v", s, check.ResponseJSONContent.Value)
				}
			case fmt.Stringer:
				if s.String() != check.ResponseJSONContent.Value {
					return Failf(check, "%v not equal to %v", s.String(), check.ResponseJSONContent.Value)
				}
			default:
				return Failf(check, "json response could not be parsed back to string")
			}
		}
		if urlObj.Scheme == "https" && check.MaxSSLExpiry > checkResults.SSLExpiry {
			return Failf(check, "SSL certificate expires soon %d > %d", checkResults.SSLExpiry, check.MaxSSLExpiry)
		}

		responseStatus.WithLabelValues(strconv.Itoa(checkResults.ResponseCode), statusCodeToClass(checkResults.ResponseCode), endpoint).Inc()
		sslExpiration.WithLabelValues(endpoint).Set(float64(checkResults.SSLExpiry))

		return &pkg.CheckResult{
			Check:    check,
			Pass:     true,
			Duration: checkResults.ResponseTime,
			Invalid:  false,
			Metrics: []pkg.Metric{
				{
					Name: "response_code",
					Type: metrics.CounterType,
					Labels: map[string]string{
						"code":     strconv.Itoa(checkResults.ResponseCode),
						"endpoint": endpoint,
					},
				},
			},
		}
	}
	return Failf(check, "No DNS results found")
}

func (c *HttpChecker) checkHTTP(urlObj pkg.URL) (*HTTPCheckResult, error) {
	var exp time.Time
	start := time.Now()
	var urlString string
	if urlObj.Port > 0 {
		urlString = fmt.Sprintf("%s://%s%s", urlObj.Scheme, net.JoinHostPort(urlObj.IP, strconv.Itoa(urlObj.Port)), urlObj.Path)
	} else {
		urlString = fmt.Sprintf("%s://%s%s", urlObj.Scheme, urlObj.IP, urlObj.Path)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         urlObj.Host,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(urlObj.Method, urlString, strings.NewReader(urlObj.Body))
	if err != nil {
		return nil, err
	}

	req.Host = urlObj.Host
	req.Header.Add("Host", urlObj.Host)
	for header, field := range urlObj.Headers {
		req.Header.Add(header, field)
	}
	if urlObj.Username != "" && urlObj.Password != "" {
		req.SetBasicAuth(urlObj.Username, urlObj.Password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.TLS != nil {
		certificates := resp.TLS.PeerCertificates
		if len(certificates) > 0 {
			exp = certificates[0].NotAfter
		}
	}
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	content := string(res)
	// logger.Tracef("GET %s => %s", urlString, content)
	sslExpireDays := int(exp.Sub(start).Hours() / 24.0)
	var sslExpiryDaysRounded int
	if sslExpireDays <= 0 {
		sslExpiryDaysRounded = 0
	} else {
		sslExpiryDaysRounded = sslExpireDays
	}

	defer resp.Body.Close()
	elapsed := time.Since(start)
	checkResult := HTTPCheckResult{
		Endpoint:     urlObj.Host,
		Record:       urlObj.IP,
		ResponseCode: resp.StatusCode,
		SSLExpiry:    sslExpiryDaysRounded,
		Content:      content,
		ResponseTime: elapsed.Milliseconds(),
	}
	return &checkResult, nil
}

func (c *HttpChecker) ParseAuth(check v1.HTTPCheck) (string, string, error) {
	kommons := c.GetClient()
	if kommons == nil {
		return "", "", errors.New("Kommons client not set for HttpChecker instance")
	}
	namespace := check.GetNamespace()
	if check.Authentication == nil {
		return "", "", nil
	}
	_, username, err := kommons.GetEnvValue(check.Authentication.Username, namespace)
	if err != nil {
		return "", "", err
	}
	_, password, err := kommons.GetEnvValue(check.Authentication.Password, namespace)
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

func DNSLookup(endpoint string) ([]pkg.URL, error) {
	if net.ParseIP(endpoint) != nil {
		return []pkg.URL{pkg.URL{IP: endpoint}}, nil
	}
	var result []pkg.URL
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if ip.To4() == nil {
			continue
		}
		port, _ := strconv.Atoi(parsedURL.Port())
		path := parsedURL.Path
		if parsedURL.RawQuery != "" {
			path += "?" + parsedURL.RawQuery
		}
		urlObj := pkg.URL{
			IP:     ip.String(),
			Port:   port,
			Host:   parsedURL.Hostname(),
			Scheme: parsedURL.Scheme,
			Path:   path,
		}
		result = append(result, urlObj)
	}

	return result, nil
}

func statusCodeToClass(statusCode int) string {
	if statusCode >= 100 && statusCode < 200 {
		return "1xx"
	} else if statusCode >= 200 && statusCode < 300 {
		return "2xx"
	} else if statusCode >= 300 && statusCode < 400 {
		return "3xx"
	} else if statusCode >= 400 && statusCode < 500 {
		return "4xx"
	} else if statusCode >= 500 && statusCode < 600 {
		return "5xx"
	} else {
		return "unknown"
	}
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
