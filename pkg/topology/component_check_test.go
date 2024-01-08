package topology

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Topology checks", ginkgo.Ordered, func() {
	topology := pkg.Topology{Name: "Topology ComponentCheck"}
	component := pkg.Component{
		Name: "Component",
		ComponentChecks: []v1.ComponentCheck{{
			Selector: types.ResourceSelector{
				Name:          "ComponentCheckSelector",
				LabelSelector: "check-target=api",
			},
		}},
	}
	canary := models.Canary{
		ID:   uuid.New(),
		Name: "Canary",
		Spec: []byte(`{"spec": {}}`),
	}

	ginkgo.BeforeAll(func() {
		err := DefaultContext.DB().Create(&topology).Error
		Expect(err).To(BeNil())

		component.TopologyID = topology.ID
		err = DefaultContext.DB().Create(&component).Error
		Expect(err).To(BeNil())

		err = DefaultContext.DB().Create(&canary).Error
		Expect(err).To(BeNil())

		check1 := pkg.Check{
			Name:     "Check-1",
			CanaryID: canary.ID,
			Labels: map[string]string{
				"check-target": "api",
				"name":         "check-1",
			},
		}
		check2 := pkg.Check{
			Name:     "Check-2",
			CanaryID: canary.ID,
			Labels: map[string]string{
				"check-target": "ui",
				"name":         "check-2",
			},
		}
		check3 := pkg.Check{
			Name:     "Check-3",
			CanaryID: canary.ID,
			Labels: map[string]string{
				"check-target": "api",
				"name":         "check-3",
			},
		}

		err = DefaultContext.DB().Create([]pkg.Check{check1, check2, check3}).Error
		Expect(err).To(BeNil())
	})

	ginkgo.It("should create check component relationships", func() {
		ComponentCheckRun.Context = DefaultContext
		ComponentCheckRun.Trace = true
		ComponentCheckRun.Run()
		expectJobToPass(ComponentCheckRun)
		cr, err := component.GetChecks(DefaultContext.DB())
		Expect(err).To(BeNil())

		// Check-1 and Check-3 should be present but not Check-2
		Expect(len(cr)).To(Equal(2))
	})
})
