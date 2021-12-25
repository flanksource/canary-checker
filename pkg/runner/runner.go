package runner

import "github.com/flanksource/canary-checker/pkg/prometheus"

var RunnerName string

var Version string

var RunnerLabels map[string]string = make(map[string]string)

var Prometheus *prometheus.PrometheusClient
