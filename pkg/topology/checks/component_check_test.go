package checks

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Test component check sync job", ginkgo.Ordered, func() {
	component := pkg.Component{
		Name: "Component",
		ComponentChecks: []v1.ComponentCheck{{
			Selector: v1.ResourceSelector{
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
		err := db.Gorm.Create(&component).Error
		Expect(err).To(BeNil())

		err = db.Gorm.Create(&canary).Error
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

		err = db.Gorm.Create([]pkg.Check{check1, check2, check3}).Error
		Expect(err).To(BeNil())
	})

	ginkgo.It("should create check component relationships", func() {
		ComponentCheckRun()
		cr, err := db.GetCheckRelationshipsForComponent(component.ID)
		Expect(err).To(BeNil())

		// Check-1 and Check-3 should be present but not Check-2
		Expect(len(cr)).To(Equal(2))
	})
})
