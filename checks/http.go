package checks

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/http"
	"github.com/flanksource/commons/http/middlewares"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/runner"
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

func (c *HTTPChecker) generateHTTPRequest(ctx *context.Context, check v1.HTTPCheck, connection *models.Connection) (*http.Request, error) {
	client := http.NewClient().UserAgent("canary-checker/" + runner.Version)

	for _, header := range check.Headers {
		value, err := ctx.GetEnvValueFromCache(header, ctx.GetNamespace())
		if err != nil {
			return nil, fmt.Errorf("failed getting header (%v): %w", header, err)
		}

		client.Header(header.Name, value)
	}

	if connection.Username != "" || connection.Password != "" {
		client.Auth(connection.Username, connection.Password)
	}

	if check.Oauth2 != nil {
		client.OAuth(middlewares.OauthConfig{
			ClientID:     connection.Username,
			ClientSecret: connection.Password,
			TokenURL:     check.Oauth2.TokenURL,
			Scopes:       check.Oauth2.Scopes,
			Params:       check.Oauth2.Params,
		})
	}

	if check.TLSConfig != nil {
		tlsconfig := http.TLSConfig{
			InsecureSkipVerify: check.TLSConfig.InsecureSkipVerify,
			HandshakeTimeout:   check.TLSConfig.HandshakeTimeout,
		}

		if !check.TLSConfig.CA.IsEmpty() {
			v, err := ctx.GetEnvValueFromCache(check.TLSConfig.CA, ctx.GetNamespace())
			if err != nil {
				return nil, fmt.Errorf("failed getting header (%v): %w", check.TLSConfig.CA, err)
			}
			tlsconfig.CA = v
		}

		if !check.TLSConfig.Cert.IsEmpty() {
			v, err := ctx.GetEnvValueFromCache(check.TLSConfig.Cert, ctx.GetNamespace())
			if err != nil {
				return nil, fmt.Errorf("failed getting header (%v): %w", check.TLSConfig.Cert, err)
			}
			tlsconfig.Cert = v
		}

		if !check.TLSConfig.Key.IsEmpty() {
			v, err := ctx.GetEnvValueFromCache(check.TLSConfig.Key, ctx.GetNamespace())
			if err != nil {
				return nil, fmt.Errorf("failed getting header (%v): %w", check.TLSConfig.Key, err)
			}
			tlsconfig.Key = v
		}

		var err error
		client, err = client.TLSConfig(tlsconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to set tls config: %w", err)
		}
	}

	client.NTLM(check.NTLM)
	client.NTLMV2(check.NTLMv2)

	if check.ThresholdMillis > 0 {
		client.Timeout(time.Duration(check.ThresholdMillis) * time.Millisecond)
	}

	// TODO: Add finer controls over tracing to the canary
	if ctx.IsTrace() && ctx.Properties()["http.trace"] != "disabled" {
		client.TraceToStdout(http.TraceAll)
		client.Trace(http.TraceAll)
	} else if ctx.IsDebug() && ctx.Properties()["http.debug"] != "disabled" {
		client.TraceToStdout(http.TraceHeaders)
		client.Trace(http.TraceHeaders)
	}

	return client.R(ctx), nil
}

func truncate(text string, max int) string {
	length := len(text)
	if length <= max {
		return text
	}
	return text[0:max]
}

func (c *HTTPChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.HTTPCheck)
	var results pkg.Results
	var err error
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	//nolint:staticcheck
	if check.Endpoint != "" && check.URL != "" {
		return results.Failf("cannot specify both endpoint and url")
	}

	//nolint:staticcheck
	if check.Endpoint != "" && check.URL == "" {
		check.URL = check.Endpoint
	}

	connection, connectionData, err := ctx.GetConnectionTemplate(check.Connection)
	if err != nil {
		return results.Invalidf("error getting connection  %v", err)
	}

	if connection.URL == "" {
		return results.Invalidf("no url or connection specified")
	}

	if ntlm, ok := connection.Properties["ntlm"]; ok {
		check.NTLM = ntlm == "true"
	} else if ntlm, ok := connection.Properties["ntlmv2"]; ok {
		check.NTLMv2 = ntlm == "true"
	}

	templateEnv := map[string]any{}

	for k, v := range ctx.Environment {
		templateEnv[k] = v
	}
	for k, v := range connectionData {
		templateEnv[k] = v
	}

	for _, env := range check.EnvVars {
		if val, err := ctx.GetEnvValueFromCache(env, ctx.GetNamespace()); err != nil {
			return results.WithError(ctx.Oops().Wrap(err)).Invalidf("failed to get env value: %v", env.Name)
		} else {
			ctx.Infof("%s => %s err:%v", env.Name, val, err)
			templateEnv[env.Name] = val
		}
	}

	ctx = ctx.WithCheck(check).WithEnvValues(templateEnv)

	oops := ctx.Oops().Hint(logger.Pretty(ctx.Environment))

	templater := ctx.NewStructTemplater(ctx.Environment, "template", nil)
	if err := templater.Walk(connection); err != nil {
		return results.WithError(oops.Wrap(err)).Invalidf("failed to template url")
	}

	uri := connection.URL

	oops = oops.With("url", uri)

	if _uri, err := url.Parse(uri); err != nil {
		return results.WithError(oops.Wrap(err)).Invalidf("invalid url  '%s'", uri)
	} else if _uri.Scheme == "" {
		return results.WithError(oops.Errorf("invalid url")).Invalidf("invalid url, missing scheme '%s'", uri)
	} else if _uri.Host == "" {
		return results.WithError(oops.Errorf("invalid url")).Invalidf("invalid url, missing host '%s'", uri)
	}

	check.URL = uri

	body := check.Body
	if check.TemplateBody {
		body, err = template(ctx, v1.Template{Template: body})
		if err != nil {
			return results.WithError(oops.Wrap(err)).Invalidf("failed to template request body: %v", err)
		}
	}

	oops = oops.Hint(body)

	request, err := c.generateHTTPRequest(ctx, check, connection)
	if err != nil {
		return results.ErrorMessage(oops.Wrap(err))
	}

	if body != "" {
		if err := request.Body(body); err != nil {
			return results.ErrorMessage(oops.Wrap(err))
		}
	}

	start := time.Now()

	ctx.Tracef("%s	%s", console.Greenf(check.GetMethod()), check.URL)

	response, err := request.Do(check.GetMethod(), check.URL)
	if err != nil {
		return results.ErrorMessage(oops.Wrap(err))
	}

	elapsed := time.Since(start)
	status := response.StatusCode

	result.AddMetric(pkg.Metric{
		Name: "response_code",
		Type: metrics.CounterType,
		Labels: map[string]string{
			"code": strconv.Itoa(status),
			"url":  check.URL,
		},
	})

	result.Duration = elapsed.Milliseconds()
	responseStatus.WithLabelValues(strconv.Itoa(status), statusCodeToClass(status), check.URL).Inc()
	age := response.GetSSLAge()
	if age != nil {
		sslExpiration.WithLabelValues(check.URL).Set(age.Hours() * 24)
	}

	data := map[string]interface{}{
		"code":    status,
		"headers": response.GetHeaders(),
		"elapsed": time.Since(start),
		"sslAge":  utils.Deref(age),
		"json":    make(map[string]any),
	}

	responseBody, err := response.AsString()
	if err != nil {
		return results.ErrorMessage(err)
	}
	data["content"] = responseBody

	if response.IsJSON() {
		var jsonContent interface{}
		if err := json.Unmarshal([]byte(responseBody), &jsonContent); err == nil {
			data["json"] = jsonContent
		} else if check.Test.IsEmpty() {
			return results.Failf("invalid json response: %v", err)
		} else {
			ctx.Tracef("ignoring invalid json response %v", err)
		}
	}

	result.AddData(data)

	if check.ResponseJSONContent != nil {
		ctx.Tracef("jsonContent is deprecated")
	}

	if ok := response.IsOK(check.ResponseCodes...); !ok {
		if len(check.ResponseCodes) == 0 {
			results.Failf("expected %d to be 200..299", status)
		} else {
			return results.Failf("expected %d to be in %v", status, check.ResponseCodes)
		}
	}

	if check.ThresholdMillis > 0 && check.ThresholdMillis < int(elapsed.Milliseconds()) {
		return results.Failf("threshold exceeded %s > %d", utils.Age(elapsed), check.ThresholdMillis)
	}

	if check.ResponseContent != "" && !strings.Contains(body, check.ResponseContent) {
		return results.Failf("expected %v, found %v", check.ResponseContent, truncate(body, 100))
	}

	if check.MaxSSLExpiry > 0 {
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
