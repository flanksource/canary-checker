package checks

import (
	gocontext "context"
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
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
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	logger.Infof("YASH: IN ALERTMANAGER")
	client := alertmanagerClient.NewHTTPClientWithConfig(nil, &alertmanagerClient.TransportConfig{
		Host: check.GetEndpoint(), Schemes: []string{"https", "http"}, BasePath: alertmanagerClient.DefaultBasePath,
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

	alertMessage := make(map[string]any)
	for _, alert := range alerts.Payload {
		name := alert.Labels["alertname"]
		alertMessage[name] = extractMessage(alert.Annotations)
	}

	for k, v := range alertMessage {
		logger.Infof("YASH ALERTMANAGER MESSAGE %s=%s", k, v)
	}
	result.AddDetails(alertMessage)
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
