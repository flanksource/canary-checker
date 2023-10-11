package checks

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

func getWithEnvironment(ctx *context.Context, r *pkg.CheckResult) *context.Context {
	r.Data["canary"] = map[string]any{
		"name":      r.Canary.GetName(),
		"namespace": r.Canary.GetNamespace(),
		"labels":    r.Canary.GetLabels(),
		"id":        r.Canary.GetPersistedID(),
	}
	r.Data["check"] = map[string]any{
		"name":        r.Check.GetName(),
		"id":          r.Canary.GetCheckID(r.Check.GetName()),
		"description": r.Check.GetDescription(),
		"labels":      r.Check.GetLabels(),
		"endpoint":    r.Check.GetEndpoint(),
		"duration":    time.Millisecond * time.Duration(r.GetDuration()),
	}
	return ctx.New(r.Data)
}

func getLabels(ctx *context.Context, metric external.Metrics) (map[string]string, []string, error) {
	var labels = make(map[string]string)
	var names = []string{}
	for _, label := range metric.Labels {
		val := label.Value
		if label.ValueExpr != "" {
			var err error
			val, err = template(ctx, v1.Template{Expression: label.ValueExpr})
			if err != nil {
				return nil, nil, err
			}
		}
		labels[label.Name] = val
		names = append(names, label.Name)
	}
	sort.Strings(names)
	return labels, names, nil
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

func exportCheckMetrics(ctx *context.Context, results pkg.Results) {
	if len(results) == 0 {
		return
	}

	for _, r := range results {
		for _, spec := range r.Check.GetMetricsSpec() {
			if spec.Name == "" || spec.Value == "" {
				continue
			}

			ctx = getWithEnvironment(ctx, r)

			var err error
			var labels map[string]string
			var labelNames []string
			if labels, labelNames, err = getLabels(ctx, spec); err != nil {
				r.ErrorMessage(err)
				continue
			}

			var collector prometheus.Collector
			var e any
			if collector, e = getOrAddPrometheusMetric(spec.Name, spec.Type, labelNames); e != nil {
				r.ErrorMessage(fmt.Errorf("failed to create metric %s (%s) %s: %s", spec.Name, spec.Type, labelNames, e))
				continue
			}

			var val float64
			if val, err = getMetricValue(ctx, spec); err != nil {
				r.ErrorMessage(err)
				continue
			}

			if ctx.IsDebug() {
				ctx.Debugf("%s%v=%0.3f", spec.Name, getLabelString(labels), val)
			}

			switch collector := collector.(type) {
			case *prometheus.HistogramVec:
				collector.With(labels).Observe(val)
			case *prometheus.GaugeVec:
				collector.With(labels).Set(val)
			case *prometheus.CounterVec:
				if val <= 0 {
					continue
				}
				collector.With(labels).Add(val)
			}
		}
	}
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
