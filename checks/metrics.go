package checks

import (
	"sort"
	"strconv"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
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
	case "guage":
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

func exportCheckMetrics(ctx *context.Context, c external.Check, results pkg.Results) {
	metricsSpec := c.GetMetricsSpec()
	if metricsSpec.Name == "" || metricsSpec.Value == "" {
		return
	}

	var collector prometheus.Collector
	var exists bool
	if collector, exists = collectorMap[metricsSpec.Name]; !exists {
		collector = addPrometheusMetric(metricsSpec.Name, metricsSpec.Type, metricsSpec.Labels)
		prometheus.MustRegister()
		if collector == nil {
			logger.Errorf("Invalid type for check.metrics %s for check[%s]", metricsSpec.Type, c.GetName())
			return
		}
	}

	tplValue := v1.Template{Template: metricsSpec.Value}
	for _, r := range results {
		valRaw, err := template(ctx.New(r.Data), tplValue)
		if err != nil {
			logger.Errorf("Error templating type for check.metrics %s for check[%s]", metricsSpec.Type, c.GetName())
		}
		val, err := strconv.ParseFloat(valRaw, 64)
		if err != nil {
			// TODO Yash
		}
		tplLabels := make(map[string]string)
		for labelKey, labelVals := range metricsSpec.Labels {
			label, err := template(ctx.New(r.Data), v1.Template{Template: labelVals})
			if err != nil {
				// TODO Yash
			}
			tplLabels[labelKey] = label
		}
		orderedLabelVals := promLabelsOrderedVals(tplLabels)

		switch collector.(type) {
		case prometheus.HistogramVec:
			h := collector.(prometheus.HistogramVec)
			// TODO Val to float64, log error
			h.WithLabelValues(orderedLabelVals...).Observe(val)
		case prometheus.GaugeVec:
			g := collector.(prometheus.GaugeVec)
			// TODO How to distinguish between gauge set vs add/sub
			g.WithLabelValues(orderedLabelVals...).Set(val)
		case prometheus.CounterVec:
			c := collector.(prometheus.CounterVec)
			if val <= 0 {
				continue
			}
			c.WithLabelValues(orderedLabelVals...).Add(val)
		}
	}
}
