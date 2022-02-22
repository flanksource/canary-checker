package checks

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
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
}

// Type: returns checker type
func (c *HTTPChecker) Type() string {
	return "http"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *HTTPChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.HTTP {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *HTTPChecker) configure(req *http.HTTPRequest, ctx *context.Context, check v1.HTTPCheck, kommons *kommons.Client) error {
	for _, header := range check.Headers {
		if kommons == nil {
			return fmt.Errorf("HTTP headers are not supported outside k8s")
		}
		key, value, err := kommons.GetEnvValue(header, ctx.Canary.GetNamespace())
		if err != nil {
			return errors.WithMessagef(err, "failed getting header: %v", header)
		}
		req.Header(key, value)
	}

	auth, err := GetAuthValues(check.Authentication, kommons, ctx.Canary.GetNamespace())
	if err != nil {
		return err
	}
	if auth != nil {
		req.Auth(auth.Username.Value, auth.Password.Value)
	}

	req.NTLM(check.NTLM)
	req.NTLMv2(check.NTLMv2)

	req.Trace(ctx.IsTrace()).Debug(ctx.IsDebug())
	return nil
}

func truncate(text string, max int) string {
	length := len(text)
	if length <= max {
		return text
	}
	return text[0:max]
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics

func (c *HTTPChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.HTTPCheck)
	var results pkg.Results
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)
	if _, err := url.Parse(check.Endpoint); err != nil {
		return results.ErrorMessage(err)
	}

	endpoint := check.Endpoint

	req := http.NewRequest(check.Endpoint).Method(check.GetMethod())

	if err := c.configure(req, ctx, check, ctx.Kommons); err != nil {
		return results.ErrorMessage(err)
	}

	resp := req.Do(check.Body)
	result.Duration = resp.Elapsed.Milliseconds()
	status := resp.GetStatusCode()
	result.AddMetric(pkg.Metric{
		Name: "response_code",
		Type: metrics.CounterType,
		Labels: map[string]string{
			"code":     strconv.Itoa(status),
			"endpoint": endpoint,
		},
	})
	responseStatus.WithLabelValues(strconv.Itoa(status), statusCodeToClass(status), endpoint).Inc()
	age := resp.GetSSLAge()
	if age != nil {
		sslExpiration.WithLabelValues(endpoint).Set(age.Hours() * 24)
	}

	body, _ := resp.AsString()

	data := map[string]interface{}{
		"code":    status,
		"headers": resp.GetHeaders(),
		"elapsed": resp.Elapsed,
		"sslAge":  age,
		"content": body,
	}
	if resp.IsJSON() {
		json, err := resp.AsJSON()
		if err != nil {
			return results.ErrorMessage(err)
		} else {
			data["json"] = json.Value
			if check.ResponseJSONContent.Path != "" {
				err := resp.CheckJSONContent(json.Value, check.ResponseJSONContent)
				if err != nil {
					return results.ErrorMessage(err)
				}
			}
		}
	}

	result.AddData(data)

	if status == -1 {
		return results.Failf("%v", truncate(resp.Error.Error(), 500))
	}

	if ok := resp.IsOK(check.ResponseCodes...); !ok {
		return results.Failf("response code invalid %d != %v", status, check.ResponseCodes)
	}

	if check.ThresholdMillis > 0 && check.ThresholdMillis < int(resp.Elapsed.Milliseconds()) {
		return results.Failf("threshold exceeded %s > %d", utils.Age(resp.Elapsed), check.ThresholdMillis)
	}

	if check.ResponseContent != "" && !strings.Contains(body, check.ResponseContent) {
		return results.Failf("expected %v, found %v", check.ResponseContent, truncate(body, 100))
	}

	if req.URL.Scheme == "https" && check.MaxSSLExpiry > 0 {
		if age == nil {
			return results.Failf("No certificate found to check age")
		}
		if *age < time.Duration(check.MaxSSLExpiry)*time.Hour*24 {
			return results.Failf("SSL certificate expires soon %s > %d", utils.Age(*age), check.MaxSSLExpiry)
		}
	}
	return results
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
