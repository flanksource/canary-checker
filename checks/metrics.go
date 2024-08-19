package checks

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/prometheus/client_golang/prometheus"
)

var collectorMap = make(map[string]prometheus.Collector)

func getOrAddPrometheusMetric(name, metricType string, labelNames []string) (collector prometheus.Collector, e any) {
	defer func() {
		e = recover()
	}()
	key := name + metricType + strings.Join(labelNames, ",")
	if collector, exists := collectorMap[key]; exists {
		return collector, nil
	}

	switch metricType {
	case "histogram":
		collector = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: name}, labelNames)
	case "counter":
		collector = prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: name}, labelNames)
	case "gauge":
		collector = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: name}, labelNames)
	default:
		return nil, fmt.Errorf("unknown metric type %s", metricType)
	}
	collectorMap[key] = collector
	return collector, prometheus.Register(collector)
}

func getLabels(ctx *context.Context, metric external.Metrics) (map[string]string, error) {
	var labels = make(map[string]string)
	for _, label := range metric.Labels {
		val := label.Value
		if label.ValueExpr != "" {
			var err error
			val, err = template(ctx, v1.Template{Expression: label.ValueExpr})
			if err != nil {
				return nil, err
			}
		}
		labels[label.Name] = val
	}

	return labels, nil
}

func getLabelNames(labels map[string]string) []string {
	var s []string

	for k := range labels {
		s = append(s, k)
	}
	sort.Strings(s)

	return s
}

func getLabelString(labels map[string]string) string {
	s := "{"
	for k, v := range labels {
		if s != "{" {
			s += ", "
		}
		s += fmt.Sprintf("%s=%s", k, v)
	}
	s += "}"

	return s
}

func ExportCheckMetrics(ctx *context.Context, results pkg.Results) {
	if len(results) == 0 {
		return
	}

	for _, r := range results {
		checkCtx := ctx.WithCheckResult(r)
		for _, metric := range r.Metrics {
			if err := exportMetric(checkCtx, metric); err != nil {
				r.ErrorMessage(err)
			}
		}
		for _, spec := range r.Check.GetMetricsSpec() {
			if spec.Name == "" || spec.Value == "" {
				continue
			}

			if metric, err := templateMetrics(checkCtx, spec); err != nil {
				r.ErrorMessage(err)
			} else if err := exportMetric(checkCtx, *metric); err != nil {
				r.ErrorMessage(err)
			}
		}
	}
}

func templateMetrics(ctx *context.Context, spec external.Metrics) (*pkg.Metric, error) {
	var val float64
	var err error
	var labels map[string]string
	if val, err = getMetricValue(ctx, spec); err != nil {
		return nil, err
	}

	if labels, err = getLabels(ctx, spec); err != nil {
		return nil, err
	}

	return &pkg.Metric{
		Name:   spec.Name,
		Type:   pkg.MetricType(spec.Type),
		Value:  val,
		Labels: labels,
	}, nil
}

func exportMetric(ctx *context.Context, spec pkg.Metric) error {
	var collector prometheus.Collector
	labelNames := getLabelNames(spec.Labels)
	var e any
	if collector, e = getOrAddPrometheusMetric(spec.Name, string(spec.Type), labelNames); e != nil {
		return fmt.Errorf("failed to create metric %s (%s) %s: %s", spec.Name, spec.Type, labelNames, e)
	}

	props := ctx.Properties()
	if ctx.IsDebug() && !props.Off("metrics.debug", false) {
		ctx.Debugf("%s%v=%0.3f", spec.Name, getLabelString(spec.Labels), spec.Value)
	} else if props.On(false, "metrics.debug") {
		ctx.Infof("%s%v=%0.3f", spec.Name, getLabelString(spec.Labels), spec.Value)
	}

	switch collector := collector.(type) {
	case *prometheus.HistogramVec:
		collector.With(spec.Labels).Observe(spec.Value)
	case *prometheus.GaugeVec:
		collector.With(spec.Labels).Set(spec.Value)
	case *prometheus.CounterVec:
		if spec.Value <= 0 {
			return nil
		}
		collector.With(spec.Labels).Add(spec.Value)
	}
	return nil
}

func getMetricValue(ctx *context.Context, spec external.Metrics) (float64, error) {
	tplValue := v1.Template{Expression: spec.Value}

	valRaw, err := template(ctx, tplValue)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseFloat(valRaw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a number", valRaw)
	}
	return val, nil
}
