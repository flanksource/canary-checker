package utils

import (
	"context"
	"fmt"
	"time"

	prometheusapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	v1.API
}

func (p PrometheusClient) GetHistogramQuantileLatency(percentile, checkKey, duration string) (value string, err error) {
	modelValue, _, err := p.Query(context.TODO(), fmt.Sprintf("histogram_quantile(%v, sum(rate(canary_check_duration_bucket{key='%v'}[%v])) by (le))", percentile, checkKey, duration), time.Now())
	if modelValue == nil {
		return "", nil
	}
	value = modelValue.(model.Vector)[0].Value.String()
	return value, err
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
