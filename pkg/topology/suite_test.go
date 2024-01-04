package topology

import (
	"testing"

	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/tests/setup"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	DefaultContext dutyContext.Context
)

func expectJobToPass(j *job.Job) {
	history, err := j.FindHistory()
	Expect(err).To(BeNil())
	Expect(len(history)).To(BeNumerically(">=", 1))
	Expect(history[0].Status).To(Equal(models.StatusSuccess))
}

func TestTopologyJobs(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Topology")
}

var _ = ginkgo.BeforeSuite(func() {
	DefaultContext = setup.BeforeSuiteFn().WithDBLogLevel("trace").WithTrace()

})
var _ = ginkgo.AfterSuite(setup.AfterSuiteFn)
