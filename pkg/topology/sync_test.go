package topology

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Test component relationship sync job", ginkgo.Ordered, func() {
	parentComponent := pkg.Component{
		Name: "Component",
		Selectors: []v1.ResourceSelector{
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
		err := db.Gorm.Create(&parentComponent).Error
		Expect(err).To(BeNil())

		err = db.Gorm.Create(pkg.Components{&childComponent1, &childComponent2, &childComponent3}).Error
		Expect(err).To(BeNil())
	})

	ginkgo.It("should create component relationships", func() {
		ComponentRun()
		relationships, err := db.GetChildRelationshipsForParentComponent(parentComponent.ID)
		Expect(err).To(BeNil())

		// Child-1 and Child-3 should be present but not Child-2
		Expect(len(relationships)).To(Equal(2))
	})

	ginkgo.It("should handle component relationship deletions", func() {
		err := db.Gorm.Delete(&childComponent3).Error
		Expect(err).To(BeNil())

		ComponentRun()
		relationships, err := db.GetChildRelationshipsForParentComponent(parentComponent.ID)
		Expect(err).To(BeNil())

		// Only child 1 should be present
		Expect(len(relationships)).To(Equal(1))

		err = db.Gorm.Create(&childComponent3).Error
		Expect(err).To(BeNil())

		ComponentRun()
		relationships, err = db.GetChildRelationshipsForParentComponent(parentComponent.ID)
		Expect(err).To(BeNil())

		// Child-1 and Child-3 should be present but not Child-2
		Expect(len(relationships)).To(Equal(2))

	})
})
