package prometheus

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/flanksource/duty/connection"
	dutyContext "github.com/flanksource/duty/context"
	prometheusapi "github.com/prometheus/client_golang/api"
	promV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheusConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	promV1.API
}

var PrometheusURL string

func (p PrometheusClient) GetHistogramQuantileLatency(percentile, checkKey, duration string) (latency float64, err error) {
	modelValue, _, err := p.Query(context.TODO(), fmt.Sprintf("histogram_quantile(%v, sum(rate(canary_check_duration_bucket{key='%v'}[%v])) by (le))", percentile, checkKey, duration), time.Now())
	if err != nil {
		return 0, err
	}
	if modelValue == nil {
		return 0, nil
	}
	return float64(modelValue.(model.Vector)[0].Value), nil
}

func (p PrometheusClient) GetUptime(checkKey, duration string) (float64, error) {
	success := fmt.Sprintf("rate(canary_check_success_count{key='%v'}[%v])", checkKey, duration)
	failed := fmt.Sprintf("rate(canary_check_failed_count{key='%v'}[%v])", checkKey, duration)
	uptime, _, err := p.Query(context.TODO(), fmt.Sprintf("%s/%s + %s", failed, failed, success), time.Now())
	if err != nil {
		return 0, err
	}
	return 100 - float64(uptime.(model.Vector)[0].Value), nil
}

func NewPrometheusAPI(ctx dutyContext.Context, conn connection.HTTPConnection) (*PrometheusClient, error) {
	if conn.URL == "" {
		return nil, nil
	}

	roundTripper := prometheusapi.DefaultRoundTripper
	if !conn.Username.IsEmpty() {
		username, err := ctx.GetEnvValueFromCache(conn.Username, ctx.GetNamespace())
		if err != nil {
			return nil, err
		}

		password, err := ctx.GetEnvValueFromCache(conn.Password, ctx.GetNamespace())
		if err != nil {
			return nil, err
		}

		roundTripper = prometheusConfig.NewBasicAuthRoundTripper(
			prometheusConfig.NewInlineSecret(username),
			prometheusConfig.NewInlineSecret(password),
			roundTripper)
	} else if !conn.OAuth.IsEmpty() {
		clientID, err := ctx.GetEnvValueFromCache(conn.OAuth.ClientID, ctx.GetNamespace())
		if err != nil {
			return nil, err
		}

		clientSecret, err := ctx.GetEnvValueFromCache(conn.OAuth.ClientSecret, ctx.GetNamespace())
		if err != nil {
			return nil, err
		}

		roundTripper = prometheusConfig.NewOAuth2RoundTripper(
			nil,
			&prometheusConfig.OAuth2{
				ClientID:       clientID,
				ClientSecret:   prometheusConfig.Secret(clientSecret),
				Scopes:         conn.OAuth.Scopes,
				TokenURL:       conn.OAuth.TokenURL,
				EndpointParams: conn.OAuth.Params,
			},
			roundTripper,
			nil)
	} else if !conn.Bearer.IsEmpty() {
		clientID, err := ctx.GetEnvValueFromCache(conn.Bearer, ctx.GetNamespace())
		if err != nil {
			return nil, err
		}

		roundTripper = prometheusConfig.NewAuthorizationCredentialsRoundTripper(
			"Bearer",
			prometheusConfig.NewInlineSecret(clientID),
			roundTripper)
	}

	transportConfig := roundTripper.(*http.Transport)
	transportConfig.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	cfg := prometheusapi.Config{
		Address:      conn.URL,
		RoundTripper: transportConfig,
	}
	client, err := prometheusapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	promapi := promV1.NewAPI(client)
	return &PrometheusClient{
		API: promapi,
	}, nil
}
