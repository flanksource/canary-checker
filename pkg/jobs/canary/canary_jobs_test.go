package canary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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
			Context: DefaultContext.WithObject(v1Canary).WithAnyValue("canary", v1Canary),
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

var _ = ginkgo.Describe("SyncCanaryJob concurrent reschedule", ginkgo.Ordered, func() {
	var (
		canaryID uuid.UUID
		specV1   types.JSON
		specV2   types.JSON
		dbCanary pkg.Canary
	)

	ginkgo.BeforeAll(func() {
		canaryCtx.DefaultContext = DefaultContext
		canaryID = uuid.New()

		specV1 = types.JSON(fmt.Sprintf(`{
			"schedule": "@every 30s",
			"http": [{
				"name": "concurrent-test",
				"endpoint": "http://127.0.0.1:1/v1"
			}]
		}`))

		specV2 = types.JSON(fmt.Sprintf(`{
			"schedule": "@every 30s",
			"http": [{
				"name": "concurrent-test",
				"endpoint": "http://127.0.0.1:1/v2"
			}]
		}`))

		model := &models.Canary{
			ID:        canaryID,
			Name:      "concurrent-reschedule-test",
			Namespace: "default",
			AgentID:   uuid.Nil,
			Source:    "kubernetes/" + canaryID.String(),
			Spec:      specV1,
		}
		Expect(DefaultContext.DB().Create(model).Error).To(BeNil())

		dbCanary = pkg.Canary{
			ID:      canaryID,
			Name:    model.Name,
			Spec:    specV1,
			Source:  model.Source,
			Namespace: model.Namespace,
		}

		// Initial sync to populate canaryJobs map and cron.
		Expect(SyncCanaryJob(DefaultContext, dbCanary)).To(BeNil())

		// Clear any entries accumulated before this spec.
		Unschedule(canaryID.String())
	})

	ginkgo.It("must create exactly 1 cron entry after concurrent reschedules", func() {
		// Use spec V2 so DeepEqual detects a change.
		dbCanaryV2 := dbCanary
		dbCanaryV2.Spec = specV2

		// Initial sync with V2 — creates a single cron entry.
		Expect(SyncCanaryJob(DefaultContext, dbCanaryV2)).To(BeNil())

		before := countCronEntriesForCanary(canaryID.String())
		Expect(before).To(Equal(1), "expected exactly 1 cron entry after initial sync")

		// Simulate concurrent reschedules.  Each goroutine changes the spec
		// and calls SyncCanaryJob again, mimicking the race between a
		// controller reconcile and the periodic SyncCanaryJobs job.
		const goroutines = 10
		var wg sync.WaitGroup
		errs := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				// Toggle between V1 and V2 so every call sees a
				// "changed" spec relative to whatever is in the map.
				c := dbCanary
				if i%2 == 0 {
					c.Spec = specV2
				} else {
					c.Spec = specV1
				}
				if err := SyncCanaryJob(DefaultContext, c); err != nil {
					errs <- err
				}
			}(i)
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			ginkgo.Fail(fmt.Sprintf("unexpected error: %v", err))
		}

		// Verify only one cron entry exists for this canary.
		after := countCronEntriesForCanary(canaryID.String())
		Expect(after).To(Equal(1),
			"expected exactly 1 cron entry after concurrent reschedules, got %d", after)
	})
})

// countCronEntriesForCanary returns the number of cron entries whose
// job carries the given canary Kubernetes UID.
func countCronEntriesForCanary(canaryUID string) int {
	count := 0
	for _, entry := range CanaryScheduler.Entries() {
		jobUID := string(entry.Job.(*job.Job).GetObjectMeta().UID)
		if strings.EqualFold(jobUID, canaryUID) {
			count++
		}
	}
	return count
}
