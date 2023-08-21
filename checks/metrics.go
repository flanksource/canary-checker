package checks

import (
	"sort"
	"strconv"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var collectorMap = make(map[string]prometheus.Collector)

func promLabelsOrderedKeys(labels map[string]string) []string {
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func promLabelsOrderedVals(labels map[string]string) []string {
	var vals []string
	keys := promLabelsOrderedKeys(labels)
	for _, k := range keys {
		vals = append(vals, labels[k])
	}
	return vals
}

func addPrometheusMetric(name, metricType string, labels map[string]string) prometheus.Collector {
	var collector prometheus.Collector
	switch metricType {
	case "histogram":
		collector = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: name},
			promLabelsOrderedKeys(labels),
		)
	case "counter":
		collector = prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: name},
			promLabelsOrderedKeys(labels),
		)
	case "gauge":
		collector = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: name},
			promLabelsOrderedKeys(labels),
		)
	default:
		return nil
	}

	collectorMap[name] = collector
	prometheus.MustRegister(collector)
	return collector
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

			var collector prometheus.Collector
			var exists bool
			if collector, exists = collectorMap[spec.Name]; !exists {
				collector = addPrometheusMetric(spec.Name, spec.Type, spec.Labels)
				if collector == nil {
					logger.Errorf("Invalid type for check.metrics %s for check[%s]", spec.Type, r.Check.GetName())
					continue
				}
			}

			tplValue := v1.Template{Template: spec.Value}
			templateInput := map[string]any{
				"result": r.Data,
				"check": map[string]any{
					"name":        r.Check.GetName(),
					"description": r.Check.GetDescription(),
					"labels":      r.Check.GetLabels(),
					"endpoint":    r.Check.GetEndpoint(),
					"duration":    r.GetDuration(),
				},
			}

			valRaw, err := template(ctx.New(templateInput), tplValue)
			if err != nil {
				logger.Errorf("Error templating value for check.metrics template %s for check[%s]: %v", spec.Value, r.Check.GetName(), err)
				continue
			}
			val, err := strconv.ParseFloat(valRaw, 64)
			if err != nil {
				logger.Errorf("Error converting value %s to float for check.metrics template %s for check[%s]: %v", valRaw, spec.Value, r.Check.GetName(), err)
				continue
			}
			tplLabels := make(map[string]string)
			for labelKey, labelVals := range spec.Labels {
				label, err := template(ctx.New(templateInput), v1.Template{Template: labelVals})
				if err != nil {
					logger.Errorf("Error templating label %s:%s for check.metrics for check[%s]: %v", labelKey, labelVals, r.Check.GetName(), err)
					continue
				}
				tplLabels[labelKey] = label
			}
			orderedLabelVals := promLabelsOrderedVals(tplLabels)

			switch collector := collector.(type) {
			case *prometheus.HistogramVec:
				collector.WithLabelValues(orderedLabelVals...).Observe(val)
			case *prometheus.GaugeVec:
				collector.WithLabelValues(orderedLabelVals...).Set(val)
			case *prometheus.CounterVec:
				if val <= 0 {
					continue
				}
				collector.WithLabelValues(orderedLabelVals...).Add(val)
			default:
				logger.Errorf("Got unknown type for check.metrics %T", collector)
			}
		}
	}
}
