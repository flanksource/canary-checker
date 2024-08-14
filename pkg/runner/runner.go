package runner

import (
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/commons/collections"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var RunnerName string

var Version string

var RunnerLabels map[string]string = make(map[string]string)

var Prometheus *prometheus.PrometheusClient

var IncludeNamespaces []string

// WatchNamespace is the kubernetes operator namespace.
var WatchNamespace string

var IncludeCanaries []string

var IncludeTypes []string

func IsCanaryIgnored(canary *metav1.ObjectMeta) bool {
	if !collections.MatchItems(canary.Namespace, IncludeNamespaces...) {
		return true
	}

	if !collections.MatchItems(canary.Name, IncludeCanaries...) {
		return true
	}

	return canary.Annotations != nil && canary.Annotations["suspend"] == "true"
}
