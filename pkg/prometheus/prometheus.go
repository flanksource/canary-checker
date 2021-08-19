package prometheus

import (
	"context"
	"fmt"
	"strconv"
	"time"

	prometheusapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	v1.API
}

func (p PrometheusClient) GetHistogramQuantileLatency(percentile, checkKey, duration string) (latency string, err error) {
	modelValue, _, err := p.Query(context.TODO(), fmt.Sprintf("histogram_quantile(%v, sum(rate(canary_check_duration_bucket{key='%v'}[%v])) by (le))", percentile, checkKey, duration), time.Now())
	if err != nil {
		return "", err
	}
	if modelValue == nil {
		return "", nil
	}
	latency = modelValue.(model.Vector)[0].Value.String()
	return latency, nil
}

func (p PrometheusClient) GetUptime(checkKey, duration string) (string, error) {
	successRateModal, _, err := p.Query(context.TODO(), fmt.Sprintf("rate(canary_check_success_count{key='%v'}[%v])", checkKey, duration), time.Now())
	if err != nil {
		return "", err
	}
	if successRateModal == nil {
		return "", err
	}
	successRate := successRateModal.(model.Vector)[0].Value.String()
	failRateModal, _, err := p.Query(context.TODO(), fmt.Sprintf("rate(canary_check_failed_count{key='%v'}[%v])", checkKey, duration), time.Now())
	if err != nil {
		return "", err
	}
	if failRateModal == nil {
		return "", err
	}
	failRate := failRateModal.(model.Vector)[0].Value.String()
	successCount, err := strconv.ParseFloat(successRate, 64)
	if err != nil {
		return "", err
	}
	failCount, err := strconv.ParseFloat(failRate, 64)
	if err != nil {
		return "", err
	}
	uptime := 100 * (1 - failCount/(successCount+failCount))
	return fmt.Sprintf("%0.1f%%", uptime), nil
}

func NewPrometheusAPI(url string) (*PrometheusClient, error) {
	if url == "" {
		return nil, nil
	}
	cfg := prometheusapi.Config{
		Address: url,
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
