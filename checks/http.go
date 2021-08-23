package checks

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/pkg/errors"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/http"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/utils"
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
func (c *HTTPChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.HTTP {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

func (c *HTTPChecker) configure(req *http.HTTPRequest, namespace string, check v1.HTTPCheck) error {
	kommons := c.GetClient()
	for _, header := range check.Headers {
		if kommons == nil {
			return fmt.Errorf("HTTP headers are not supported outside k8s")
		}
		key, value, err := kommons.GetEnvValue(header, namespace)
		if err != nil {
			return errors.WithMessagef(err, "failed getting header: %v", header)
		}
		req.Header(key, value)
	}

	auth, err := GetAuthValues(check.Authentication, kommons, namespace)
	if err != nil {
		return err
	}
	if auth != nil {
		req.Auth(auth.Username.Name, auth.Password.Value)
	}
	req.NTLM(check.NTLM)

	if logger.IsDebugEnabled() {
		req.Debug(true)
	} else if logger.IsTraceEnabled() {
		req.Trace(true)
	}
	return nil
}

func truncate(text string, max int) string {
	return text[0:int(math.Min(float64(max), float64(len(text)-1)))]
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics

func (c *HTTPChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.HTTPCheck)
	result := pkg.Success(check)
	if _, err := url.Parse(check.Endpoint); err != nil {
		return result.ErrorMessage(err)
	}

	namespace := ctx.Canary.GetNamespace()
	endpoint := check.Endpoint

	req := http.NewRequest(check.Endpoint).Method(check.Method)
	if err := c.configure(req, namespace, check); err != nil {
		return result.ErrorMessage(err)
	}

	resp := req.Do(check.Body)
	result.Duration = resp.Elapsed.Milliseconds()
	result.AddMetric(pkg.Metric{
		Name: "response_code",
		Type: metrics.CounterType,
		Labels: map[string]string{
			"code":     strconv.Itoa(resp.StatusCode),
			"endpoint": endpoint,
		},
	})
	responseStatus.WithLabelValues(strconv.Itoa(resp.StatusCode), statusCodeToClass(resp.StatusCode), endpoint).Inc()
	age := resp.GetSSLAge()
	if age != nil {
		sslExpiration.WithLabelValues(endpoint).Set(age.Hours() * 24)
	}

	body, _ := resp.AsString()
	defer resp.Body.Close()

	data := map[string]interface{}{
		"code":    resp.StatusCode,
		"headers": resp.Header,
		"elapsed": resp.Elapsed,
		"sslAge":  age,
		"content": body,
	}
	if resp.IsJSON() {
		json, err := resp.AsJSON()
		if err != nil {
			result.ErrorMessage(err)
		} else {
			data["json"] = json.Value
		}
	}

	result.AddData(data)

	if ok := resp.IsOK(check.ResponseCodes...); !ok {
		return result.Failf("response code invalid %d != %v", resp.StatusCode, check.ResponseCodes)
	}

	if check.ThresholdMillis > 0 && check.ThresholdMillis < int(resp.Elapsed.Milliseconds()) {
		return result.Failf("threshold exceeded %s > %d", utils.Age(resp.Elapsed), check.ThresholdMillis)
	}

	if check.ResponseContent != "" && !strings.Contains(body, check.ResponseContent) {
		return result.Failf("expected %v, found %v", check.ResponseContent, truncate(body, 100))
	}

	if req.URL.Scheme == "https" && check.MaxSSLExpiry > 0 {
		if age == nil {
			return result.Failf("No certificate found to check age")
		}
		if *age < time.Duration(check.MaxSSLExpiry)*time.Hour*24 {
			return result.Failf("SSL certificate expires soon %s > %d", utils.Age(*age), check.MaxSSLExpiry)
		}
	}
	return result
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
