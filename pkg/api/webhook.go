package api

import (
	goctx "context"
	"fmt"
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

func WebhookHandler(c echo.Context) error {
	id := c.Param("id")

	body := make(map[string]any)
	if err := c.Bind(&body); err != nil {
		return WriteError(c, Errorf(EINVALID, "invalid request body: %v", err))
	}

	authToken := c.QueryParam("token")
	if authToken == "" {
		authToken = c.Request().Header.Get("Webhook-Token")
	}

	if err := webhookHandler(c.Request().Context(), id, authToken, body); err != nil {
		return WriteError(c, err)
	}

	return c.JSON(http.StatusOK, &HTTPSuccess{Message: "ok"})
}

func webhookHandler(ctx goctx.Context, id, authToken string, body map[string]any) error {
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
		token, err := duty.GetEnvValueFromCache(nil, *webhook.Token, canary.Namespace) // TODO: K8s dependency
		if err != nil {
			return err
		}

		if token != "" && token != authToken {
			return Errorf(EUNAUTHORIZED, "invalid webhook token")
		}
	}

	// TODO: For alert manager, the alerts are in body["alerts"].
	// We probably need to make the field configurable.

	// We create the check from the request's body ??
	var results pkg.Results
	result := pkg.Success(webhook, *canary)
	results = append(results, result)
	result.AddDetails(body["alerts"])

	scrapeCtx := context.New(nil, nil, db.Gorm, db.Pool, *canary)
	transformedResults := checks.TransformResults(scrapeCtx, results)
	results = append(results, transformedResults...)

	checks.ExportCheckMetrics(scrapeCtx, transformedResults)
	_ = checks.ProcessResults(scrapeCtx, results)
	return nil
}
