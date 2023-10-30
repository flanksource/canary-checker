package api

import (
	goctx "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/models"
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

	if err := webhookHandler(c.Request().Context(), id, authToken, data); err != nil {
		return WriteError(c, err)
	}

	return c.JSON(http.StatusOK, &HTTPSuccess{Message: "ok"})
}

func webhookHandler(ctx goctx.Context, id, authToken string, data CheckData) error {
	webhookChecks, err := db.FindChecks(context.DefaultContext.Wrap(ctx), id, checks.WebhookCheckType)
	if err != nil {
		return err
	}

	var check models.Check
	if len(webhookChecks) == 0 {
		return Errorf(ENOTFOUND, "check (%s) not found", id)
	} else if len(webhookChecks) > 1 {
		return Errorf(EINVALID, "multiple checks with name: %s were found. Please use the check id or modify the check to have a unique name", id)
	} else {
		check = webhookChecks[0]
	}

	var canary *v1.Canary
	if c, err := db.FindCanaryByID(check.CanaryID.String()); err != nil {
		return fmt.Errorf("failed to get canary: %w", err)
	} else if c == nil {
		return Errorf(ENOTFOUND, "canary was not found (id:%s): %v", check.CanaryID.String(), err)
	} else if canary, err = c.ToV1(); err != nil {
		return err
	}

	webhook := canary.Spec.Webhook
	if webhook == nil {
		return Errorf(ENOTFOUND, "no webhook checks found")
	}

	// Authorization
	if webhook.Token != nil {
		token, err := duty.GetEnvValueFromCache(context.DefaultContext.Kubernetes(), *webhook.Token, canary.Namespace)
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

	scrapeCtx := context.New(context.DefaultContext.Kommons(), context.DefaultContext.Kubernetes(), db.Gorm, db.Pool, *canary)
	transformedResults := checks.TransformResults(scrapeCtx, results)
	results = append(results, transformedResults...)

	checks.ExportCheckMetrics(scrapeCtx, transformedResults)
	_ = checks.ProcessResults(scrapeCtx, results)

	// TODO: persist these results
	return nil
}
