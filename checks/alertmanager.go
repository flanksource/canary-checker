package checks

import (
	gocontext "context"
	"fmt"
	"net/url"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	alertmanagerClient "github.com/prometheus/alertmanager/api/v2/client"
	alertmanagerAlert "github.com/prometheus/alertmanager/api/v2/client/alert"
)

type AlertManagerChecker struct{}

func (c *AlertManagerChecker) Type() string {
	return "alertmanager"
}

func (c *AlertManagerChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.AlertManager {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *AlertManagerChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.AlertManagerCheck)
	var results pkg.Results
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	parsedURL, err := url.Parse(connection.URL)
	if err != nil {
		return results.Failf("error parsing url: %v", err)
	}
	client := alertmanagerClient.NewHTTPClientWithConfig(nil, &alertmanagerClient.TransportConfig{
		Host:     parsedURL.Host,
		Schemes:  []string{parsedURL.Scheme},
		BasePath: alertmanagerClient.DefaultBasePath,
	})
	var filters []string
	for k, v := range check.Filters {
		filters = append(filters, fmt.Sprintf("%s=~%s", k, v))
	}
	for _, alert := range check.Alerts {
		filters = append(filters, fmt.Sprintf("alertname=~%s", alert))
	}
	for _, ignore := range check.Ignore {
		filters = append(filters, fmt.Sprintf("alertname!~%s", ignore))
	}
	for k, v := range check.ExcludeFilters {
		filters = append(filters, fmt.Sprintf("%s!=%s", k, v))
	}

	alerts, err := client.Alert.GetAlerts(&alertmanagerAlert.GetAlertsParams{
		Context: gocontext.Background(),
		Filter:  filters,
	})
	if err != nil {
		results.ErrorMessage(fmt.Errorf("Error fetching from alertmanager: %v", err))
		return results
	}

	type Alerts struct {
		Alerts []map[string]interface{} `json:"alerts,omitempty"`
	}

	var alertMessages []map[string]interface{}
	for _, alert := range alerts.Payload {
		alertMap := map[string]any{
			"name":        generateFullName(alert.Labels["alertname"], alert.Labels),
			"message":     extractMessage(alert.Annotations),
			"labels":      alert.Labels,
			"annotations": alert.Annotations,
			"fingerprint": *alert.Fingerprint,
		}
		alertMessages = append(alertMessages, alertMap)
	}

	result.AddDetails(Alerts{Alerts: alertMessages})
	return results
}

func extractMessage(annotations map[string]string) string {
	keys := []string{"message", "description", "summary"}
	for _, key := range keys {
		if val, exists := annotations[key]; exists {
			return val
		}
	}
	return ""
}

func generateFullName(name string, labels map[string]string) string {
	fullName := []string{name}

	// We add alert metadata to the check name
	level1 := []string{"namespace", "node"}
	for _, key := range level1 {
		if labels[key] != "" {
			fullName = append(fullName, labels[key])
		}
	}

	// Only one of these labels must be used
	level2 := []string{"deployment", "daemonset", "statefulset", "cronjob_name", "job_name", "pod", "nodename"}
	for _, key := range level2 {
		if labels[key] != "" {
			fullName = append(fullName, labels[key])
			break
		}
	}

	// Add container name if it exists
	if labels["container"] != "" && labels["job"] != labels["container"] {
		fullName = append(fullName, labels["container"])
	}

	return strings.Join(fullName, "/")
}
