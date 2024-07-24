package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/labstack/echo/v4"
)

const webhookBodyLimit = 10 * 1024 // 10 MB

type CheckData struct {
	Headers map[string]string `json:"headers"`
	JSON    map[string]any    `json:"json,omitempty"`
	Content string            `json:"content,omitempty"`
}

func WebhookHandler(c echo.Context) error {
	id := c.Param("id")

	authToken := c.QueryParam("token")
	if authToken == "" {
		authToken = c.Request().Header.Get("Webhook-Token")
	}

	data := CheckData{
		Headers: make(map[string]string),
	}
	for k := range c.Request().Header {
		data.Headers[k] = c.Request().Header.Get(k)
	}

	if c.Request().Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(c.Request().Body).Decode(&data.JSON); err != nil {
			return WriteError(c, err)
		}
	} else {
		b, err := io.ReadAll(io.LimitReader(c.Request().Body, webhookBodyLimit))
		if err != nil {
			return WriteError(c, err)
		}

		data.Content = string(b)
	}

	ctx := c.Request().Context().(dutyContext.Context)

	if err := webhookHandler(ctx, id, authToken, data); err != nil {
		return WriteError(c, err)
	}

	return c.JSON(http.StatusOK, &HTTPSuccess{Message: "ok"})
}

func webhookHandler(ctx dutyContext.Context, id, authToken string, data CheckData) error {
	canaries, err := db.FindCanariesByWebhook(ctx, id)
	if err != nil {
		return err
	} else if len(canaries) == 0 {
		return nil
	} else if len(canaries) > 1 {
		return fmt.Errorf("multiple canaries found for webhook: %s", id)
	}

	canary, err := canaries[0].ToV1()
	if err != nil {
		return err
	}

	webhook := canary.Spec.Webhook
	if webhook == nil {
		return Errorf(ENOTFOUND, "no webhook checks found")
	}

	// Authorization
	if webhook.Token != nil && !webhook.Token.IsEmpty() {
		token, err := ctx.GetEnvValueFromCache(*webhook.Token, canary.Namespace)
		if err != nil {
			return err
		}

		if token != "" && token != authToken {
			return Errorf(EUNAUTHORIZED, "invalid webhook token")
		}
	}

	result := pkg.Success(webhook, *canary)
	result.AddDetails(data)

	results := []*pkg.CheckResult{result}

	scrapeCtx := context.New(ctx, *canary)
	transformedResults := checks.TransformResults(scrapeCtx, results)

	checks.ExportCheckMetrics(scrapeCtx, transformedResults)
	for _, result := range transformedResults {
		_, err := cache.PostgresCache.Add(ctx, pkg.FromV1(result.Canary, result.Check), pkg.CheckStatusFromResult(*result))
		if err != nil {
			return err
		}
	}

	return nil
}
