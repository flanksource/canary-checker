package canary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	canaryCtx "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
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
		CanaryScheduler.Stop()
		Expect(requestCount).To(Equal(1))
	})
})
