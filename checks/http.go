package checks

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/http"
	"github.com/flanksource/commons/http/middlewares"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/gocolly/colly/v2"
	"github.com/samber/lo"
	"github.com/samber/oops"
	"k8s.io/client-go/rest"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
)

const trueString = "true"

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

func (c *HTTPChecker) generateHTTPRequest(ctx *context.Context, check *v1.HTTPCheck, connection *models.Connection) (*http.Request, error) {
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

	if check.Kubernetes != nil {
		k8s, err := ctx.Kubernetes()
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate k8s client (%s): %w", check.Kubernetes, err)
		}

		k8srt, err := rest.TransportFor(k8s.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to get transport config for k8s: %w", err)
		}

		client.Use(func(rt nethttp.RoundTripper) nethttp.RoundTripper {
			return k8srt
		})

		parsedURL, err := url.Parse(check.URL)
		if err != nil {
			return nil, fmt.Errorf("error parsing check url[%s]: %w", check.URL, err)
		}

		port := lo.CoalesceOrEmpty(parsedURL.Port(), lo.Ternary(parsedURL.Scheme == "https", "443", "80"))
		parts := strings.Split(parsedURL.Hostname(), ".")
		if len(parts) < 2 {
			return nil, fmt.Errorf("check host[%s] is invalid. Use `service.namespace` format", parsedURL.Hostname())
		}
		svc, ns := parts[0], parts[1]
		check.URL = fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%s/proxy/%s", k8s.Config.Host, ns, svc, port, strings.TrimPrefix(parsedURL.Path, "/"))
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
		}
		tlsconfig.HandshakeTimeout, _ = check.TLSConfig.HandshakeTimeout.GetDurationOr(time.Second * 10)

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

func hydrate(ctx *context.Context, check v1.HTTPCheck) (*v1.HTTPCheck, *models.Connection, oops.OopsErrorBuilder, pkg.Results) {
	var results pkg.Results

	var err error
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	oops := ctx.Oops()

	if check.Kubernetes != nil {
		*ctx = ctx.WithKubernetesConnection(*check.Kubernetes)
		if _, _, err := check.Kubernetes.Populate(ctx.Context, true); err != nil {
			return nil, nil, oops, results.Invalidf("failed to hydrate kubernetes connection: %v", err)
		}
	}

	//nolint:staticcheck
	if check.Endpoint != "" && check.URL != "" {
		return nil, nil, oops, results.Invalidf("cannot specify both endpoint and url")
	}

	//nolint:staticcheck
	if check.Endpoint != "" && check.URL == "" {
		check.URL = check.Endpoint
	}

	connection, connectionData, err := ctx.GetConnectionTemplate(check.Connection)
	if err != nil {
		return nil, nil, oops, results.Invalidf("error getting connection  %v", err)
	}

	if connection.URL == "" {
		return nil, nil, oops, results.Invalidf("no url or connection specified")
	}

	if ntlm, ok := connection.Properties["ntlm"]; ok {
		check.NTLM = ntlm == trueString
	} else if ntlm, ok := connection.Properties["ntlmv2"]; ok {
		check.NTLMv2 = ntlm == trueString
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
			return nil, nil, oops, results.WithError(ctx.Oops().Wrap(err)).Invalidf("failed to get env value: %v", env.Name)
		} else {
			templateEnv[env.Name] = val
		}
	}

	ctx = ctx.WithCheck(check).WithEnvValues(templateEnv)

	oops = ctx.Oops().Hint(logger.Pretty(ctx.Environment))

	templater := ctx.NewStructTemplater(ctx.Environment, "template", nil)
	if err := templater.Walk(connection); err != nil {
		return nil, nil, oops, results.WithError(oops.Wrap(err)).Invalidf("failed to template url")
	}

	uri := connection.URL

	oops = oops.With("url", uri)

	if _uri, err := url.Parse(uri); err != nil {
		return nil, nil, oops, results.WithError(oops.Wrap(err)).Invalidf("invalid url  '%s'", uri)
	} else if _uri.Scheme == "" {
		return nil, nil, oops, results.WithError(oops.Errorf("invalid url")).Invalidf("invalid url, missing scheme '%s'", uri)
	} else if _uri.Host == "" {
		return nil, nil, oops, results.WithError(oops.Errorf("invalid url")).Invalidf("invalid url, missing host '%s'", uri)
	} else if _uri.User != nil {
		connection.Username = _uri.User.Username()
		connection.Password, _ = _uri.User.Password()
		_uri.User = nil
		uri = _uri.String()
	}

	check.URL = uri

	body := check.Body
	if check.TemplateBody {
		body, err = template(ctx, v1.Template{Template: body})
		if err != nil {
			return nil, nil, oops, results.WithError(oops.Wrap(err)).Invalidf("failed to template request body: %v", err)
		}
	}

	oops = oops.Hint(body)
	check.Body = body

	return &check, connection, oops, results
}

type CrawlResults struct {
	Missing []string `json:"missing"`
	Visited int      `json:"visited"`
}

func crawl(ctx *context.Context, check v1.HTTPCheck, results *pkg.Results) {
	c := colly.NewCollector(colly.Async(), colly.MaxDepth(lo.CoalesceOrEmpty(check.Crawl.Depth, 10)))
	rule := colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: lo.CoalesceOrEmpty(check.Crawl.Parallelism, 2),
	}
	var err error
	rule.Delay, err = check.Crawl.Delay.GetDurationOr(500 * time.Millisecond)
	if err != nil {
		results.Invalidf("delay is invalid duration: %v ", check.Crawl.Delay)
		return
	}
	rule.RandomDelay, err = check.Crawl.RandomDelay.GetDurationOr(100 * time.Millisecond)
	if err != nil {
		results.Invalidf("randomDelay is invalid duration: %v", check.Crawl.RandomDelay)
		return
	}

	if check.Crawl.Depth > 0 {
		c.MaxDepth = check.Crawl.Depth
	}

	ctx.Tracef("%s %s depth=%d, parallelism=%d, delay=%v", console.Greenf("CRAWL"), check.URL, c.MaxDepth, rule.Parallelism, rule.Delay)
	if err := c.Limit(&rule); err != nil {
		results.Invalidf("%v", err.Error())
		return
	}

	if len(check.Crawl.AllowedDomains) > 0 {
		c.AllowedDomains = check.Crawl.AllowedDomains
	}
	if len(check.Crawl.DisallowedDomains) > 0 {
		c.DisallowedDomains = check.Crawl.DisallowedDomains
	}
	for _, i := range check.Crawl.DisallowedURLFilters {
		if re, err := regexp.Compile(i); err == nil {
			c.DisallowedURLFilters = append(c.DisallowedURLFilters, re)
		} else {
			results.Invalidf("invalid regex %s", i)
		}
	}

	for _, i := range check.Crawl.AllowedURLFilters {
		if re, err := regexp.Compile(i); err == nil {
			c.URLFilters = append(c.URLFilters, re)
		} else {
			results.Invalidf("invalid regex %s", i)
		}
	}

	u, _ := url.Parse(check.URL)
	if len(c.AllowedDomains) == 0 && len(c.DisallowedDomains) == 0 {
		c.AllowedDomains = []string{u.Hostname()}
	}

	ctx.Tracef("allowedDomains: %v, disallowedDomains: %v, allowedURLFilters: %v, disallowedURLFilters: %v", c.AllowedDomains, c.DisallowedDomains, c.URLFilters, c.DisallowedURLFilters)

	errors := sync.Map{}
	c.OnError(func(r *colly.Response, err error) {
		errors.Store(r.Request.URL.String(), 1)
	})

	c.OnResponse(func(r *colly.Response) {
		if r.Trace != nil {
			ctx.Debugf("%s -> %d firstByte=%s connect=%s", r.Request.URL, r.StatusCode, r.Trace.FirstByteDuration, r.Trace.ConnectDuration)
		} else {
			ctx.Debugf("%s -> %d", r.Request.URL, r.StatusCode)
		}
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		ctx.Logger.V(4).Infof("Visiting %s from %s", e.Attr("href"), e.Request.URL)
		_ = e.Request.Visit(e.Attr("href"))
	})
	c.OnScraped(func(r *colly.Response) {
		ctx.Tracef("Crawled %s", r.Request.URL)
	})
	visited := atomic.Int32{}
	c.OnRequest(func(r *colly.Request) {
		ctx.Tracef("Visiting %s", r.URL)
		visited.Add(1)
	})

	if err := c.Visit(check.URL); err != nil {
		results.Failf("%v", err.Error())
		return
	}

	c.Wait()

	data := CrawlResults{Visited: int(visited.Load()), Missing: []string{}}
	errors.Range(func(key, value any) bool {
		data.Missing = append(data.Missing, key.(string))
		return true
	})
	sort.Strings(data.Missing)

	msg := fmt.Sprintf("Visited: %d", visited.Load())
	(*results)[0].AddDataStruct(data)
	if len(data.Missing) > 0 {
		msg += fmt.Sprintf(", Missing: %d", len(data.Missing))
		results.Failf("%s", msg)
	} else {
		(*results)[0].ResultMessage("%s", msg)
	}
}

func (c *HTTPChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	var err error

	check, connection, oops, results := hydrate(ctx, extConfig.(v1.HTTPCheck))
	if check == nil {
		return results
	}

	result := results[0]

	request, err := c.generateHTTPRequest(ctx, check, connection)
	if err != nil {
		return results.ErrorMessage(oops.Wrap(err))
	}
	logger.Infof("Check url is %s", check.URL)
	start := time.Now()

	if check.Crawl != nil {
		crawl(ctx, *check, &results)
		return results
	}

	if check.Body != "" {
		if err := request.Body(check.Body); err != nil {
			return results.ErrorMessage(oops.Wrap(err))
		}
	}

	ctx.Tracef("%s	%s", console.Greenf("%s", check.GetMethod()), check.URL)

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
		sslExpiration.WithLabelValues(check.URL).Set(age.Hours() / 24)
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

	if check.ResponseContent != "" && !strings.Contains(responseBody, check.ResponseContent) {
		return results.Failf("expected %v, found %v", check.ResponseContent, pkg.TruncateMessage(responseBody))
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
