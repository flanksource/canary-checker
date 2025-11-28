package canary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	canaryCtx "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/tests/setup"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Canary Job sync", ginkgo.Ordered, func() {
	var canarySpec v1.CanarySpec
	ginkgo.BeforeEach(func() {
		canarySpec = v1.CanarySpec{
			Schedule: "@every 1s",
			HTTP: []v1.HTTPCheck{
				{
					Endpoint:      fmt.Sprintf("http://localhost:%d?delay=10", testEchoServerPort), // server only responds after 10 seconds
					ResponseCodes: []int{http.StatusOK},
				},
			},
		}
	})

	ginkgo.It("should save a canary spec", func() {
		b, err := json.Marshal(canarySpec)
		Expect(err).To(BeNil())

		var spec types.JSON
		err = json.Unmarshal(b, &spec)
		Expect(err).To(BeNil())

		canaryM := &models.Canary{
			ID: uuid.New(),
			Annotations: map[string]string{
				"trace": "true",
			},
			Spec: spec,
			Name: "http check",
		}
		err = DefaultContext.DB().Create(canaryM).Error
		Expect(err).To(BeNil())

		response, err := db.GetAllCanariesForSync(DefaultContext, "")
		Expect(err).To(BeNil())
		Expect(len(response)).To(BeNumerically(">=", 1))
	})

	ginkgo.It("schedule the canary job", func() {
		MinimumTimeBetweenCanaryRuns = 0 // reset this for now so it doesn't hinder test with small schedules
		SyncCanaryJobs.Context = DefaultContext
		canaryCtx.DefaultContext = DefaultContext
		SyncCanaryJobs.Run()
		setup.ExpectJobToPass(SyncCanaryJobs)
	})

	ginkgo.It("should verify that the endpoint wasn't called more than once after 3 seconds", func() {
		time.Sleep(time.Second * 3)
		// The job will be called on first schedule and all concurrent jobs would be aborted
		Expect(requestCount).To(Equal(1))
	})
})

var _ = ginkgo.Describe("Transformed Canary", ginkgo.Ordered, func() {
	transformExpr := `'{"name":"transformed-canary","namespace":"default","spec":{"http":[{"name":"http-check","endpoint":"https://example.com"}]}}'`
	parentCanary := pkg.Canary{
		ID:        uuid.New(),
		Name:      "canary-to-create-canary",
		Namespace: "default",
		Spec: []byte(fmt.Sprintf(`{
				"http": [{
					"name": "http-check",
					"endpoint": "https://example.com",
					"transform": {
						"expr": %q
					}
				}]
			}`, transformExpr)),
	}

	ginkgo.It("Check should create a canary", func() {
		err := DefaultContext.DB().Save(&parentCanary).Error
		Expect(err).To(BeNil())

		v1Canary, err := parentCanary.ToV1()
		Expect(err).To(BeNil())

		c := CanaryJob{
			Canary:   *v1Canary,
			DBCanary: parentCanary,
		}

		j := &job.Job{
			Name:    "Canary",
			Context: DefaultContext.WithObject(v1Canary.ObjectMeta).WithAnyValue("canary", v1Canary),
			Fn:      c.Run,
		}

		j.Run()

		var transformedCanary models.Canary
		err = DefaultContext.DB().Where("name = ? AND namespace = ?", "transformed-canary", "default").First(&transformedCanary).Error
		Expect(err).To(BeNil())
		Expect(transformedCanary.DeletedAt).To(BeNil())
		Expect(transformedCanary.Source).ToNot(BeEmpty())

	})

	ginkgo.It("should mark transformed canaries as deleted when parent canary is deleted", func() {
		// Delete the parent canary
		err := db.DeleteCanary(DefaultContext, parentCanary.ID.String())
		Expect(err).To(BeNil())

		// Verify transformed canary has deleted_at set
		var transformedCanary models.Canary
		err = DefaultContext.DB().Where("name = ? AND namespace = ?", "transformed-canary", "default").First(&transformedCanary).Error
		Expect(err).To(BeNil())
		Expect(transformedCanary.DeletedAt).ToNot(BeNil())
	})
})
