package prometheus

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	prometheusapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	v1.API
}

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

func NewPrometheusAPI(url string) (*PrometheusClient, error) {
	if url == "" {
		return nil, nil
	}
	transportConfig := prometheusapi.DefaultRoundTripper.(*http.Transport)
	transportConfig.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	cfg := prometheusapi.Config{
		Address:      url,
		RoundTripper: transportConfig,
	}
	client, err := prometheusapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	promapi := v1.NewAPI(client)
	return &PrometheusClient{
		API: promapi,
	}, nil
}
