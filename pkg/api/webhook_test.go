package api_test

import (
	"encoding/json"
	"fmt"
	netHTTP "net/http"
	"time"

	"github.com/flanksource/duty/tests/setup"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/http"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

type Alert struct {
	Status       string            `json:"status"`
	Name         string            `json:"name"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
	EndsAt       *time.Time        `json:"endsAt,omitempty"`
}

type DummyWebhookMessage struct {
	Version string  `json:"version"`
	Alerts  []Alert `json:"alerts"`
}

var testData []struct {
	Msg   Alert
	Count int
}

var canarySpec v1.CanarySpec

var client *http.Client
var canaryM *models.Canary

var _ = ginkgo.Describe("API Canary Webhook", ginkgo.Ordered, func() {

	ginkgo.BeforeAll(func() {
		// Create an array of test data with three different Alert data
		testData = []struct {
			Msg   Alert
			Count int
		}{
			{
				Msg: Alert{
					Status:       "firing",
					Name:         "ServerDown",
					Labels:       map[string]string{"severity": "critical", "alertName": "ServerDown", "location": "DataCenterA"},
					Annotations:  map[string]string{"summary": "Server in DataCenterA is down", "description": "This alert indicates that a server in DataCenterA is currently down."},
					StartsAt:     "2023-10-30T08:00:00Z",
					GeneratorURL: "http://example.com/generatorURL/serverdown",
					Fingerprint:  "a1b2c3d4e5f6",
				},
				Count: 1,
			},
			{
				Msg: Alert{
					Status:       "firing",
					Name:         "ServerUp",
					Labels:       map[string]string{"severity": "info", "alertName": "ServerUp", "location": "DataCenterB"},
					Annotations:  map[string]string{"summary": "Server in DataCenterB is up", "description": "This alert indicates that a server in DataCenterB is currently up."},
					StartsAt:     "2023-10-31T10:00:00Z",
					GeneratorURL: "http://example.com/generatorURL/serverup",
					Fingerprint:  "x1y2z3w4v5u6",
				},
				Count: 2,
			},
			{
				Msg: Alert{
					Status:       "firing",
					Name:         "HighTraffic",
					Labels:       map[string]string{"severity": "major", "alertName": "HighTraffic", "location": "DataCenterC"},
					Annotations:  map[string]string{"summary": "High traffic detected in DataCenterC", "description": "This alert indicates a high level of network traffic in DataCenterC."},
					StartsAt:     "2023-11-01T15:00:00Z",
					GeneratorURL: "http://example.com/generatorURL/hightraffic",
					Fingerprint:  "q1r2s3t4u5v6",
				},
				Count: 3,
			},
		}

		canarySpec = v1.CanarySpec{
			Schedule: "@every 1s",
			HTTP: []v1.HTTPCheck{ // Run another transformed check on the same canary to test that the "delete transform" strategy doesn't delete webhook checks
				{
					Description: v1.Description{
						Name: "my-http",
					},
					Connection: v1.Connection{
						URL: fmt.Sprintf("http://localhost:%d/http-check", testEchoServerPort),
					},
					Templatable: v1.Templatable{
						Transform: v1.Template{
							Expression: `
							json.alerts.map(r,
								{
									'name': r.name,
									'icon': r.icon,
									'message': r.message,
									'description': r.description,
									'deletedAt': has(r.deleted_at) ? r.deleted_at : null,
								}
							).toJSON()`,
						},
					},
				},
			},
			Webhook: &v1.WebhookCheck{
				Description: v1.Description{
					Name: "my-webhook",
				},
				Templatable: v1.Templatable{
					Transform: v1.Template{
						Expression: `
						results.json.alerts.map(r,
							{
								'name': r.name + r.fingerprint,
								'labels': r.labels,
								'icon': 'alert',
								'message': r.annotations.summary,
								'description': r.annotations.description,
								'deletedAt': has(r.endsAt) ? r.endsAt : null,
							}
						).toJSON()`,
					},
				},
				Token: &types.EnvVar{
					ValueStatic: "my-token",
				},
			},
		}

		client = http.NewClient().BaseURL(fmt.Sprintf("http://localhost:%d", testEchoServerPort)).Header("Content-Type", "application/json").TraceToStdout(http.TraceAll)

	})
	ginkgo.It("should save a canary spec", func() {
		b, err := json.Marshal(canarySpec)
		Expect(err).To(BeNil())

		var spec types.JSON
		err = json.Unmarshal(b, &spec)
		Expect(err).To(BeNil())

		canaryM = &models.Canary{
			ID:   uuid.New(),
			Spec: spec,
			Annotations: map[string]string{
				"trace": "true",
			},
			Name: "alert-manager-canary",
		}
		err = ctx.DB().Create(canaryM).Error
		Expect(err).To(BeNil())

		response, err := db.GetAllCanariesForSync(ctx, "")
		Expect(err).To(BeNil())
		Expect(lo.CountBy(response, func(c pkg.Canary) bool { return c.Name == canaryM.Name })).To(Equal(1))
	})

	ginkgo.It("schedule the canary job", func() {
		canaryJobs.MinimumTimeBetweenCanaryRuns = 0 // reset this for now so it doesn't hinder test with small schedules
		canaryJobs.SyncCanaryJobs.Context = ctx
		canaryJobs.SyncCanaryJobs.Run()
		setup.ExpectJobToPass(canaryJobs.SyncCanaryJobs)
	})

	ginkgo.It("Should have created the webhook check", func() {

		Eventually(func() int {
			var checks []models.Check
			_ = ctx.DB().Where("name = ?", canarySpec.Webhook.Name).Find(&checks).Error
			return len(checks)
		}, "5s", "50ms").Should(BeNumerically(">=", 1))

	})

	ginkgo.It("Should forbid when webhook is called without the auth token", func() {
		resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s", canarySpec.Webhook.Name), nil)
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(netHTTP.StatusUnauthorized))
	})

	ginkgo.It("Should call webhook with one alert at a time", func() {
		for _, td := range testData {
			resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s?token=%s", canarySpec.Webhook.Name, canarySpec.Webhook.Token.ValueStatic), DummyWebhookMessage{
				Version: "4",
				Alerts:  []Alert{td.Msg},
			})
			Expect(err).To(BeNil())
			Expect(resp.StatusCode).To(Equal(netHTTP.StatusOK))

			var result []models.Check
			err = ctx.DB().Where("type = ?", checks.WebhookCheckType).Where("deleted_at IS NULL").Where("name != ?", canarySpec.Webhook.Name).Find(&result).Error
			Expect(err).To(BeNil())
			Expect(len(result)).To(Equal(td.Count), "should have created new webhook check")
		}
	})

	ginkgo.It("Should have created the transformed http check", func() {
		Eventually(func() int {
			var transformedChecks []models.Check
			err := ctx.DB().Where("transformed = true").Where("type = 'http'").Find(&transformedChecks).Error
			Expect(err).To(BeNil())
			return len(transformedChecks)
		}, "5s", "100ms").Should(Equal(2))
	})

	ginkgo.It("Should have deleted one resolved alert from", func() {
		td := testData[0]
		td.Msg.Status = "resolved"
		td.Msg.EndsAt = utils.Ptr(time.Now())

		resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s?token=%s", canarySpec.Webhook.Name, canarySpec.Webhook.Token.ValueStatic), DummyWebhookMessage{
			Version: "4",
			Alerts:  []Alert{td.Msg},
		})
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(netHTTP.StatusOK))

		var result models.Check
		err = ctx.DB().Where("name = ?", td.Msg.Name+td.Msg.Fingerprint).Find(&result).Error
		Expect(err).To(BeNil())
		Expect(result.DeletedAt).To(Not(BeNil()))
	})

	ginkgo.It("should have deleted the transformed http check", func() {

		Eventually(func() int { return httpCheckCallCounter }, "5s", "50ms").Should(BeNumerically(">=", 1))

		logger.Debugf("http check endpoint was called %d times", httpCheckCallCounter)
		var result models.Check
		err := ctx.DB().Where("name = 'http-check'").Find(&result).Error
		Expect(err).To(BeNil())
		Expect(result.DeletedAt).To(Not(BeNil()))
	})

	ginkgo.It("should have two active and one resolved webhook check", func() {
		var activeChecks []models.Check
		err := ctx.DB().Where("type = ?", checks.WebhookCheckType).Where("deleted_at IS NULL").Where("name != ?", canarySpec.Webhook.Name).Find(&activeChecks).Error
		Expect(err).To(BeNil())
		Expect(len(activeChecks)).To(Equal(2), "There should have been 2 active webhook check")

		var deletedChecks []models.Check
		err = ctx.DB().Where("type = ?", checks.WebhookCheckType).Where("deleted_at IS NOT NULL").Where("name != ?", canarySpec.Webhook.Name).Find(&deletedChecks).Error
		Expect(err).To(BeNil())
		Expect(len(deletedChecks)).To(Equal(1), "There should have been 1 deleted webhook check")
	})
})
