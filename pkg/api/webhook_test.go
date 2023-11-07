package api_test

import (
	"encoding/json"
	"fmt"
	netHTTP "net/http"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/commons/http"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Test Sync Canary Job", ginkgo.Ordered, func() {
	canarySpec := v1.CanarySpec{
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

	var canaryM *models.Canary
	client := http.NewClient().BaseURL(fmt.Sprintf("http://localhost:%d", testEchoServerPort)).Header("Content-Type", "application/json")

	ginkgo.It("should save a canary spec", func() {
		b, err := json.Marshal(canarySpec)
		Expect(err).To(BeNil())

		var spec types.JSON
		err = json.Unmarshal(b, &spec)
		Expect(err).To(BeNil())

		canaryM = &models.Canary{
			ID:   uuid.New(),
			Spec: spec,
			Name: "alert-manager-canary",
		}
		err = testDB.Create(canaryM).Error
		Expect(err).To(BeNil())

		response, err := db.GetAllCanariesForSync(ctx, "")
		Expect(err).To(BeNil())
		Expect(len(response)).To(Equal(1))
	})

	ginkgo.It("schedule the canary job", func() {
		canaryJobs.MinimumTimeBetweenCanaryRuns = 0 // reset this for now so it doesn't hinder test with small schedules

		canaryJobs.CanaryScheduler.Start()
		jobCtx := job.JobRuntime{
			Context: ctx,
		}

		err := canaryJobs.SyncCanaryJobs(jobCtx)
		Expect(err).To(BeNil())
	})

	ginkgo.It("Should have created the webhook check", func() {
		var count = 30
		for {
			time.Sleep(time.Second) // Wait for SyncCanaryJob to create the check
			count--

			var checks []models.Check
			err := ctx.DB().Where("name = ?", canarySpec.Webhook.Name).Find(&checks).Error
			Expect(err).To(BeNil())

			if len(checks) == 1 {
				break
			}

			if len(checks) != 1 && count <= 0 {
				ginkgo.Fail("expected check to be created")
			}
		}
	})

	ginkgo.It("Should have created the transformed http check", func() {
		var transformedChecks []models.Check
		err := ctx.DB().Where("transformed = true").Where("type = 'http'").Find(&transformedChecks).Error
		Expect(err).To(BeNil())
		Expect(len(transformedChecks)).To(Equal(1))
	})

	ginkgo.It("Should forbid when webhook is called without the auth token", func() {
		resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s", canarySpec.Webhook.Name), nil)
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(netHTTP.StatusUnauthorized))
	})

	ginkgo.It("Should allow when webhook is called with the auth token", func() {
		body := `{
  "version": "4",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "name": "first",
      "labels": {
        "severity": "critical",
        "alertName": "ServerDown",
        "location": "DataCenterA"
      },
      "annotations": {
        "summary": "Server in DataCenterA is down",
        "description": "This alert indicates that a server in DataCenterA is currently down."
      },
      "startsAt": "2023-10-30T08:00:00Z",
      "generatorURL": "http://example.com/generatorURL/serverdown",
      "fingerprint": "a1b2c3d4e5f6"
    },
    {
      "status": "open",
      "labels": {
        "severity": "warning",
        "alertName": "HighCPUUsage",
        "location": "DataCenterB"
      },
      "annotations": {
        "summary": "High CPU Usage in DataCenterB",
        "description": "This alert indicates that there was high CPU usage in DataCenterB, but it is now resolved."
      },
      "startsAt": "2023-10-30T09:00:00Z",
      "generatorURL": "http://example.com/generatorURL/highcpuusage", 
      "name": "second",
      "fingerprint": "x1y2z3w4v5"
    }
  ]
}`
		resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s?token=%s", canarySpec.Webhook.Name, canarySpec.Webhook.Token.ValueStatic), body)
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(netHTTP.StatusOK))
	})

	ginkgo.It("Should have created 2 new checks from the webhook", func() {
		var result []models.Check
		err := testDB.Where("type = ?", checks.WebhookCheckType).Where("name != ?", canarySpec.Webhook.Name).Find(&result).Error
		Expect(err).To(BeNil())
		Expect(len(result)).To(Equal(2))
	})

	ginkgo.It("Should have deleted one resolved alert from", func() {
		body := `{
	"version": "4",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "name": "first",
      "labels": {
        "severity": "critical",
        "alertName": "ServerDown",
        "location": "DataCenterA"
      },
      "annotations": {
        "summary": "Server in DataCenterA is down",
        "description": "This alert indicates that a server in DataCenterA is currently down."
      },
      "startsAt": "2023-10-30T08:00:00Z",
      "generatorURL": "http://example.com/generatorURL/serverdown",
      "fingerprint": "a1b2c3d4e5f6",
      "endsAt": "2023-10-30T09:15:00Z"
    }
  ]
	}`
		resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s?token=%s", canarySpec.Webhook.Name, canarySpec.Webhook.Token.ValueStatic), body)
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(netHTTP.StatusOK))

		var result models.Check
		err = testDB.Where("name = 'firsta1b2c3d4e5f6'").Find(&result).Error
		Expect(err).To(BeNil())
		Expect(result.DeletedAt).To(Not(BeNil()))
	})

	ginkgo.It("Should create 1 new check from the webhook and deleted one", func() {
		body := `
		{
			"version": "4",
			"status": "firing",
			"alerts": [
				{
					"status": "resolved",
					"labels": {
						"severity": "warning",
						"alertName": "HighCPUUsage",
						"location": "DataCenterB"
					},
					"annotations": {
						"summary": "High CPU Usage in DataCenterB",
						"description": "This alert indicates that there was high CPU usage in DataCenterB, but it is now resolved."
					},
					"startsAt": "2023-10-30T09:00:00Z",
					"generatorURL": "http://example.com/generatorURL/highcpuusage", 
					"name": "second",
					"fingerprint": "x1y2z3w4v5",
					"endsAt": "2023-11-30T09:15:00Z"
				},
				{
					"status": "open",
					"labels": {
						"severity": "warning",
						"alertName": "HighCPUUsage",
						"location": "DataCenterB"
					},
					"annotations": {
						"summary": "High CPU Usage in DataCenterB",
						"description": "This alert indicates that there was high CPU usage in DataCenterB, but it is now resolved."
					},
					"startsAt": "2023-10-30T09:00:00Z",
					"generatorURL": "http://example.com/generatorURL/highcpuusage", 
					"name": "third",
					"fingerprint": "abc"
				}
			]
		}`
		resp, err := client.R(ctx).Post(fmt.Sprintf("/webhook/%s?token=%s", canarySpec.Webhook.Name, canarySpec.Webhook.Token.ValueStatic), body)
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(netHTTP.StatusOK))

		var result models.Check
		err = testDB.Where("name = 'thirdabc'").Find(&result).Error
		Expect(err).To(BeNil())
		Expect(result.DeletedAt).To(BeNil())

		var activeChecks []models.Check
		err = testDB.Where("type = ?", checks.WebhookCheckType).Where("deleted_at IS NULL").Where("name != ?", canarySpec.Webhook.Name).Find(&activeChecks).Error
		Expect(err).To(BeNil())
		Expect(len(activeChecks)).To(Equal(1), "There should have been just one active webhook check")
	})

	ginkgo.It("should have deleted the transformed http check", func() {
		for {
			if httpCheckCallCounter <= 1 {
				time.Sleep(time.Second)
				continue
			}

			break
		}

		logger.Debugf("http check endpoint was called %d times", httpCheckCallCounter)
		var result models.Check
		err := testDB.Where("name = 'http-check'").Find(&result).Error
		Expect(err).To(BeNil())
		Expect(result.DeletedAt).To(Not(BeNil()))
	})
})
