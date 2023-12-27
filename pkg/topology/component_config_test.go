package topology

import (
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/types"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Topology configs", ginkgo.Ordered, func() {
	component := pkg.Component{
		Name: "Component with configs",
		Configs: types.ConfigQueries{
			{
				Tags: map[string]string{
					"environment": "production",
				},
			},
		},
	}

	ginkgo.BeforeAll(func() {
		err := DefaultContext.DB().Save(&component).Error
		Expect(err).To(BeNil())
	})

	ginkgo.It("should create relationships", func() {
		ComponentConfigRun.Context = DefaultContext
		ComponentConfigRun.Trace = true
		ComponentConfigRun.Run()
		expectJobToPass(ComponentConfigRun)

		cr, err := component.GetConfigs(DefaultContext.DB())
		Expect(err).To(BeNil())

		ci, err := duty.FindCachedConfig(DefaultContext, cr[0].ConfigID.String())
		Expect(err).To(BeNil())

		tags := *ci.Tags
		Expect(tags["environment"]).To(Equal("production"))

		Expect(len(cr)).Should(BeNumerically(">", 2))
	})
})
