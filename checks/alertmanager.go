package checks

import (
	gocontext "context"
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty"
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

	if check.ConnectionName != "" {
		if ctx.Kommons == nil {
			return results.Failf("kommons client is not configured. cannot retrieve connection.")
		}

		k8sClient, err := ctx.Kommons.GetClientset()
		if err != nil {
			return results.Failf("error getting k8s client from kommons client: %v", err)
		}

		if connection, err := duty.HydratedConnectionByURL(ctx, db.Gorm, k8sClient, ctx.Namespace, check.ConnectionName); err != nil {
			return results.Failf("failed to find connection from %q: %v", check.ConnectionName, err)
		} else if connection != nil {
			check.Host = connection.URL
		}
	}

	client := alertmanagerClient.NewHTTPClientWithConfig(nil, &alertmanagerClient.TransportConfig{
		Host:     check.GetEndpoint(),
		Schemes:  []string{"http", "https"},
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

	alerts, err := client.Alert.GetAlerts(&alertmanagerAlert.GetAlertsParams{
		Context: gocontext.Background(),
		Filter:  filters,
	})
	if err != nil {
		results.ErrorMessage(fmt.Errorf("Error fetching from alertmanager: %v", err))
		return results
	}

	var alertMessages []map[string]any
	for _, alert := range alerts.Payload {
		alertMap := map[string]any{
			"name":        alert.Labels["alertname"],
			"message":     extractMessage(alert.Annotations),
			"labels":      alert.Labels,
			"annotations": alert.Annotations,
			"fingerprint": *alert.Fingerprint,
		}
		alertMessages = append(alertMessages, alertMap)
	}

	result.AddDetails(alertMessages)
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
