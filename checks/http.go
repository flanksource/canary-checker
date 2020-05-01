package checks

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/pkg"
)

var (
	dnsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_http_dns_failed",
		Help: "The total number of dns requests failed",
	})

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
	prometheus.MustRegister(dnsFailed, responseStatus, sslExpiration)
}

type HttpChecker struct{}

// Type: returns checker type
func (c *HttpChecker) Type() string {
	return "http"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *HttpChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.HTTP {
		for _, result := range c.Check(conf.HTTPCheck) {
			results <- result
		}
	}

}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *HttpChecker) Check(check pkg.HTTPCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult
	for _, endpoint := range check.Endpoints {
		rcOK, contentOK, timeOK, sslOK := false, false, false, false
		lookupResult, err := DNSLookup(endpoint)
		if err != nil {
			dnsFailed.Inc()
			checkResult := &pkg.CheckResult{
				Check:    check,
				Pass:     false,
				Invalid:  true,
				Endpoint: endpoint,
				Message:  "Failed to resolve DNS",
				Metrics:  []pkg.Metric{},
			}
			result = append(result, checkResult)
			continue
		}
		for _, urlObj := range lookupResult {
			checkResults, err := c.checkHTTP(urlObj)
			if err != nil {
				checkResult := &pkg.CheckResult{
					Check:    check,
					Pass:     false,
					Invalid:  true,
					Message:  fmt.Sprintf("%s", err),
					Endpoint: endpoint,
					Metrics:  []pkg.Metric{},
				}
				result = append(result, checkResult)
				continue
			}
			for _, rc := range check.ResponseCodes {
				if rc == checkResults.ResponseCode {
					rcOK = true
				}
			}

			contentOK = check.ResponseContent == "" || strings.Contains(checkResults.Content, check.ResponseContent)
			timeOK = check.ThresholdMillis >= int(checkResults.ResponseTime)
			sslOK = urlObj.Scheme == "http" || check.MaxSSLExpiry <= checkResults.SSLExpiry

			pass := rcOK && contentOK && timeOK && sslOK
			var msg []string

			if !rcOK {
				msg = append(msg, fmt.Sprintf("response code invalid %d != %v", checkResults.ResponseCode, check.ResponseCodes))
			}
			if !timeOK {
				msg = append(msg, fmt.Sprintf("threshold exceeeded %d > %d", checkResults.ResponseTime, check.ThresholdMillis))
			}
			if !contentOK {
				msg = append(msg, "content not found")
			}
			if !sslOK {
				msg = append(msg, fmt.Sprintf("SSL certificate expires soon %d > %d", checkResults.SSLExpiry, check.MaxSSLExpiry))
			}
			m := []pkg.Metric{
				{
					Name: "response_code",
					Type: pkg.CounterType,
					Labels: map[string]string{
						"code":     strconv.Itoa(checkResults.ResponseCode),
						"endpoint": endpoint,
					},
				},
			}
			checkResult := &pkg.CheckResult{
				Check:    check,
				Pass:     pass,
				Duration: checkResults.ResponseTime,
				Endpoint: endpoint,
				Invalid:  false,
				Message:  strings.Join(msg, ","),
				Metrics:  m,
			}
			result = append(result, checkResult)

			responseStatus.WithLabelValues(strconv.Itoa(checkResults.ResponseCode), statusCodeToClass(checkResults.ResponseCode), endpoint).Inc()
			sslExpiration.WithLabelValues(endpoint).Set(float64(checkResults.SSLExpiry))

		}

	}
	return result
}

func (c *HttpChecker) checkHTTP(urlObj pkg.URL) (*pkg.HTTPCheckResult, error) {
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
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, err
	}

	req.Host = urlObj.Host
	req.Header.Add("Host", urlObj.Host)
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
	sslExpireDays := int(exp.Sub(start).Hours() / 24.0)
	var sslExpiryDaysRounded int
	if sslExpireDays <= 0 {
		sslExpiryDaysRounded = 0
	} else {
		sslExpiryDaysRounded = sslExpireDays
	}

	defer resp.Body.Close()
	elapsed := time.Since(start)
	checkResult := pkg.HTTPCheckResult{
		Endpoint:     urlObj.Host,
		Record:       urlObj.IP,
		ResponseCode: resp.StatusCode,
		SSLExpiry:    sslExpiryDaysRounded,
		Content:      content,
		ResponseTime: elapsed.Milliseconds(),
	}
	return &checkResult, nil
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
		urlObj := pkg.URL{
			IP:     ip.String(),
			Port:   port,
			Host:   parsedURL.Hostname(),
			Scheme: parsedURL.Scheme,
			Path:   parsedURL.Path,
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
