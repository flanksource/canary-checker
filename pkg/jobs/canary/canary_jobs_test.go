package canary

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Test Sync Canary Job", ginkgo.Ordered, func() {
	canarySpec := v1.CanarySpec{
		Schedule: "@every 1s",
		HTTP: []v1.HTTPCheck{
			{
				Endpoint:      fmt.Sprintf("http://localhost:%d?delay=10", testEchoServerPort), // server only responds after 10 secodns
				ResponseCodes: []int{http.StatusOK},
			},
		},
	}

	ginkgo.It("should save a canary spec", func() {
		b, err := json.Marshal(canarySpec)
		Expect(err).To(BeNil())

		var spec types.JSON
		err = json.Unmarshal(b, &spec)
		Expect(err).To(BeNil())

		canaryM := &models.Canary{
			ID:   uuid.New(),
			Spec: spec,
			Name: "http check",
		}
		err = db.Gorm.Create(canaryM).Error
		Expect(err).To(BeNil())

		ctx := context.NewContext(gocontext.Background()).WithDB(db.Gorm, db.Pool)
		response, err := db.GetAllCanariesForSync(ctx, "")
		Expect(err).To(BeNil())
		Expect(len(response)).To(Equal(1))
	})

	ginkgo.It("schedule the canary job", func() {
		CanaryScheduler.Start()
		minimumTimeBetweenCanaryRuns = 0 // reset this for now so it doesn't hinder test with small schedules
		ctx := context.NewContext(gocontext.Background()).WithDB(db.Gorm, db.Pool)
		jobCtx := job.JobRuntime{
			Context: ctx,
		}
		err := SyncCanaryJobs(jobCtx)
		Expect(err).To(BeNil())
	})

	ginkgo.It("should verify that the endpoint wasn't called more than once after 3 seconds", func() {
		time.Sleep(time.Second * 3)
		CanaryScheduler.Stop()
		Expect(requestCount).To(Equal(1))
	})
})
