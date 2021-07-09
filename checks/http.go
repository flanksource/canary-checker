package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/commons/text"

	"github.com/Azure/go-ntlmssp"
	"github.com/flanksource/kommons"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

type HTTPChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

// Type: returns checker type
func (c *HTTPChecker) Type() string {
	return "http"
}

func (c *HTTPChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c HTTPChecker) GetClient() *kommons.Client {
	return c.kommons
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *HTTPChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.HTTP {
		results = append(results, c.Check(conf))
	}
	return results
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *HTTPChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.HTTPCheck)
	endpoint := check.Endpoint
	namespace := check.Namespace
	specNamespace := check.GetNamespace()
	var textResults bool
	if check.GetDisplayTemplate() != "" {
		textResults = true
	}

	if endpoint == "" && namespace == "" {
		return TextFailf(check, textResults, "One of Namespace or Endpoint must be specified")
	} else if endpoint != "" && namespace != "" {
		return TextFailf(check, textResults, "Namespace and Endpoint are mutually exclusive, only one may be specified")
	}
	if namespace == "*" {
		namespace = metav1.NamespaceAll
	}
	var lookupResult []pkg.URL
	if endpoint != "" {
		var err error
		lookupResult, err = DNSLookup(endpoint)
		if err != nil {
			return TextFailf(check, textResults, "failed to resolve DNS for %s", endpoint)
		}
	} else {
		k8sClient, err := pkg.NewK8sClient()
		if err != nil {
			return TextFailf(check, textResults, fmt.Sprintf("Unable to connect to k8s: %v", err))
		}
		serviceList, err := k8sClient.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return TextFailf(check, textResults, fmt.Sprintf("failed to obtain service list for namespace %v: %v", namespace, err))
		}
		for _, service := range serviceList.Items {
			endPoints, err := k8sClient.CoreV1().Endpoints(namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
			if err != nil {
				return TextFailf(check, textResults, fmt.Sprintf("Failed to obtain endpoints for service %v: %v", service.Name, err))
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
		return TextFailf(check, textResults, "Failed to lookup authentication info %v:", err)
	}
	var headers map[string]string
	kommons := c.GetClient()
	for _, header := range check.Headers {
		if kommons == nil {
			return TextFailf(check, textResults, "Kommons client not set for HTTPChecker instance")
		}
		key, value, err := kommons.GetEnvValue(header, specNamespace)
		if err != nil {
			return TextFailf(check, textResults, "Failed to parse header value: %v", err)
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
		checkResults, err := c.checkHTTP(urlObj, check.NTLM)
		if err != nil {
			return TextFailf(check, textResults, err.Error())
		}
		var results = map[string]interface{}{"code": strconv.Itoa(checkResults.ResponseCode), "content": checkResults.Content, "header": headers}
		var message string
		rcOK := false
		for _, rc := range check.ResponseCodes {
			if rc == checkResults.ResponseCode {
				rcOK = true
			}
		}
		if check.GetDisplayTemplate() != "" {
			message, err = text.TemplateWithDelims(check.GetDisplayTemplate(), "[[", "]]", results)
		}
		if !rcOK {
			failMessage := fmt.Sprintf("\nresponse code invalid %d != %v", checkResults.ResponseCode, check.ResponseCodes)
			return TextFailf(check, textResults, message+failMessage)
		}

		if check.ThresholdMillis > 0 && check.ThresholdMillis < int(checkResults.ResponseTime) {
			failMessage := fmt.Sprintf("\nthreshold exceeded %d > %d", checkResults.ResponseTime, check.ThresholdMillis)
			return TextFailf(check, textResults, message+failMessage)
		}
		if check.ResponseContent != "" && !strings.Contains(checkResults.Content, check.ResponseContent) {
			failMessage := fmt.Sprintf("\nExpected %v, found %v", check.ResponseContent, checkResults.Content)
			return TextFailf(check, textResults, message+failMessage)
		}
		if check.ResponseJSONContent.Path != "" {
			var jsonContent interface{}
			if err = json.Unmarshal([]byte(checkResults.Content), &jsonContent); err != nil {
				failMessage := fmt.Sprintf("\nCould not unmarshal response for json check: %v ", err)
				return TextFailf(check, textResults, message+failMessage)
			}

			jsonResult, err := jsonpath.Get(check.ResponseJSONContent.Path, jsonContent)
			if err != nil {
				failMessage := fmt.Sprintf("\nCould not extract path %v from response %v: %v", check.ResponseJSONContent.Path, jsonContent, err)
				return TextFailf(check, textResults, message+failMessage)
			}
			switch s := jsonResult.(type) {
			case string:
				if s != check.ResponseJSONContent.Value {
					failMessage := fmt.Sprintf("\n%v not equal to %v", s, check.ResponseJSONContent.Value)
					return TextFailf(check, textResults, message+failMessage)
				}
			case fmt.Stringer:
				if s.String() != check.ResponseJSONContent.Value {
					failMessage := fmt.Sprintf("\n%v not equal to %v", s.String(), check.ResponseJSONContent.Value)
					return TextFailf(check, textResults, message+failMessage)
				}
			default:
				return TextFailf(check, textResults, message+"\njson response could not be parsed back to string")
			}
		}
		if urlObj.Scheme == "https" && check.MaxSSLExpiry > checkResults.SSLExpiry {
			failMessage := fmt.Sprintf("\nSSL certificate expires soon %d > %d", checkResults.SSLExpiry, check.MaxSSLExpiry)
			return TextFailf(check, textResults, failMessage)
		}

		responseStatus.WithLabelValues(strconv.Itoa(checkResults.ResponseCode), statusCodeToClass(checkResults.ResponseCode), endpoint).Inc()
		sslExpiration.WithLabelValues(endpoint).Set(float64(checkResults.SSLExpiry))

		if !textResults {
			return &pkg.CheckResult{ // nolint: staticcheck
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
		return &pkg.CheckResult{ // nolint: staticcheck
			Check:       check,
			Pass:        true,
			Duration:    checkResults.ResponseTime,
			Invalid:     false,
			DisplayType: "Text",
			Message:     message,
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

func (c *HTTPChecker) checkHTTP(urlObj pkg.URL, ntlm bool) (*HTTPCheckResult, error) {
	var exp time.Time
	start := time.Now()
	var urlString string
	if urlObj.Port > 0 {
		urlString = fmt.Sprintf("%s://%s%s", urlObj.Scheme, net.JoinHostPort(urlObj.IP, strconv.Itoa(urlObj.Port)), urlObj.Path)
	} else {
		urlString = fmt.Sprintf("%s://%s%s", urlObj.Scheme, urlObj.IP, urlObj.Path)
	}
	client := getHTTPClient(urlObj.Host, ntlm)
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

func (c *HTTPChecker) ParseAuth(check v1.HTTPCheck) (string, string, error) {
	kommons := c.GetClient()
	if kommons == nil {
		return "", "", errors.New("Kommons client not set for HTTPChecker instance")
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

func getHTTPClient(urlHost string, ntlm bool) *http.Client {
	transport := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         urlHost,
		},
	}
	checkRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if ntlm {
		return &http.Client{
			Transport: ntlmssp.Negotiator{
				RoundTripper: transport,
			},
			CheckRedirect: checkRedirect,
		}
	}
	return &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
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
