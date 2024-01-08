package topology

import (
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/types"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Topology relationships", ginkgo.Ordered, func() {
	topology := pkg.Topology{Name: "Topology ComponentRelationship"}
	parentComponent := pkg.Component{
		Name: "Component",
		Selectors: []types.ResourceSelector{
			{
				Name:          "ComponentSelector",
				LabelSelector: "service=payments",
			},
		},
	}
	childComponent1 := pkg.Component{
		Name:   "Child-1",
		Labels: map[string]string{"service": "payments"},
	}
	childComponent2 := pkg.Component{
		Name:   "Child-2",
		Labels: map[string]string{"service": "logistics"},
	}
	childComponent3 := pkg.Component{
		Name:   "Child-3",
		Labels: map[string]string{"service": "payments"},
	}

	ginkgo.BeforeAll(func() {
		err := DefaultContext.DB().Create(&topology).Error
		Expect(err).To(BeNil())

		parentComponent.TopologyID = topology.ID
		ComponentRelationshipSync.Context = DefaultContext
		err = DefaultContext.DB().Create(&parentComponent).Error
		Expect(err).To(BeNil())

		childComponent1.TopologyID = topology.ID
		childComponent2.TopologyID = topology.ID
		childComponent3.TopologyID = topology.ID
		err = DefaultContext.DB().Create(pkg.Components{&childComponent1, &childComponent2, &childComponent3}).Error
		Expect(err).To(BeNil())
	})

	ginkgo.It("should create component relationships", func() {
		ComponentRelationshipSync.Run()
		expectJobToPass(ComponentRelationshipSync)

		relationships, err := parentComponent.GetChildren(DefaultContext.DB())
		Expect(err).To(BeNil())

		// Child-1 and Child-3 should be present but not Child-2
		Expect(len(relationships)).To(Equal(2))
	})

	ginkgo.It("should handle component relationship deletions", func() {
		err := DefaultContext.DB().Delete(&childComponent3).Error
		Expect(err).To(BeNil())

		ComponentRelationshipSync.Run()
		expectJobToPass(ComponentRelationshipSync)

		relationships, err := parentComponent.GetChildren(DefaultContext.DB())
		Expect(err).To(BeNil())

		// Only child 1 should be present
		Expect(len(relationships)).To(Equal(1))

		err = DefaultContext.DB().Create(&childComponent3).Error
		Expect(err).To(BeNil())

		ComponentRelationshipSync.Run()
		expectJobToPass(ComponentRelationshipSync)

		relationships, err = parentComponent.GetChildren(DefaultContext.DB())
		Expect(err).To(BeNil())

		// Child-1 and Child-3 should be present but not Child-2
		Expect(len(relationships)).To(Equal(2))
	})

	ginkgo.It("should handle component label updates", func() {
		childComponent3.Labels = map[string]string{"service": "logistics"}
		err := DefaultContext.DB().Save(&childComponent3).Error
		Expect(err).To(BeNil())
		ComponentRelationshipSync.Run()
		expectJobToPass(ComponentRelationshipSync)

		relationships, err := parentComponent.GetChildren(DefaultContext.DB())
		Expect(err).To(BeNil())

		// Only child 1 should be present as we updated the labels
		// and old relationship should be deleted
		Expect(len(relationships)).To(Equal(1))
	})
})
