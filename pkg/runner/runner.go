package runner

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/commons/collections"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var RunnerName string

var Version string

var RunnerLabels map[string]string = make(map[string]string)

var Prometheus *prometheus.PrometheusClient

// WatchNamespace is the kubernetes operator namespace.
var WatchNamespace string

var (
	// Only canaries matching these namespace will be allowed run
	IncludeNamespaces []string

	// Only canaries matching these labels will be allowed run
	IncludeLabels []string

	// Only canaries with these names will be allowed run
	IncludeCanaries []string
)

func IsCanaryIgnored(canary *metav1.ObjectMeta) bool {
	if !collections.MatchItems(canary.Namespace, IncludeNamespaces...) {
		return true
	}

	if !collections.MatchItems(canary.Name, IncludeCanaries...) {
		return true
	}

	labelSelector := collections.KeyValueSliceToMap(IncludeLabels)
	for k, v := range labelSelector {
		if lVal, ok := canary.Labels[k]; !ok {
			return true
		} else if !collections.MatchItems(lVal, v) {
			return true
		}
	}

	return false
}

func IsCanarySuspended(c v1.Canary) bool {
	return (c.Spec.Replicas != nil && *c.Spec.Replicas == 0) ||
		(c.ObjectMeta.Annotations != nil && c.ObjectMeta.Annotations["suspend"] == "true")
}
