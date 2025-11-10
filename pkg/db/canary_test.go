package db

import (
	"time"

	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/flanksource/canary-checker/checks"
)

var _ = ginkgo.Describe("Canary DB", ginkgo.Ordered, func() {
	var (
		// Test check IDs - we'll add these to LogisticsAPICanary
		httpTransformedID       uuid.UUID
		dnsTransformedID        uuid.UUID
		webhookTransformedID    uuid.UUID
		deletedTransformedID    uuid.UUID
		postgresUntransformedID uuid.UUID

		dummyCanary = models.Canary{
			ID:        uuid.New(),
			Name:      "my-random-canary-xyz-123",
			Namespace: "logistics",
			Spec:      []byte("{}"),
		}
	)

	ginkgo.BeforeAll(func() {
		Expect(DefaultContext.DB().Create(&dummyCanary).Error).To(BeNil())

		// Add transformed checks to the existing LogisticsAPICanary for testing
		httpTransformedID = uuid.New()
		err := DefaultContext.DB().Create(&models.Check{
			ID:          httpTransformedID,
			CanaryID:    dummyCanary.ID,
			Name:        "transformed-http-check",
			Type:        "http",
			Transformed: true,
		}).Error
		Expect(err).To(BeNil())

		dnsTransformedID = uuid.New()
		err = DefaultContext.DB().Create(&models.Check{
			ID:          dnsTransformedID,
			CanaryID:    dummyCanary.ID,
			Name:        "transformed-dns-check",
			Type:        "dns",
			Transformed: true,
		}).Error
		Expect(err).To(BeNil())

		webhookTransformedID = uuid.New()
		err = DefaultContext.DB().Create(&models.Check{
			ID:          webhookTransformedID,
			CanaryID:    dummyCanary.ID,
			Name:        "transformed-webhook-check",
			Type:        checks.WebhookCheckType,
			Transformed: true,
		}).Error
		Expect(err).To(BeNil())

		// Create a deleted transformed check (should be excluded)
		deletedTransformedID = uuid.New()
		err = DefaultContext.DB().Create(&models.Check{
			ID:          deletedTransformedID,
			CanaryID:    dummyCanary.ID,
			Name:        "deleted-transformed-check",
			Type:        "tcp",
			Transformed: true,
			DeletedAt:   lo.ToPtr(time.Now()),
		}).Error
		Expect(err).To(BeNil())

		// Create an untransformed check (should be excluded)
		postgresUntransformedID = uuid.New()
		err = DefaultContext.DB().Create(&models.Check{
			ID:          postgresUntransformedID,
			CanaryID:    dummyCanary.ID,
			Name:        "untransformed-postgres-check",
			Type:        "postgres",
			Transformed: false,
		}).Error
		Expect(err).To(BeNil())
	})

	ginkgo.AfterAll(func() {
		err := DefaultContext.DB().Delete(&models.Check{}, "id IN ?", []uuid.UUID{
			httpTransformedID,
			dnsTransformedID,
			webhookTransformedID,
			deletedTransformedID,
			postgresUntransformedID,
		}).Error
		Expect(err).To(BeNil())

		err = DefaultContext.DB().Delete(&dummyCanary).Error
		Expect(err).To(BeNil())
	})

	ginkgo.Describe("GetTransformedCheckIDs", func() {
		ginkgo.It("should return all transformed non-deleted checks when no types are excluded", func() {
			ids, err := GetTransformedCheckIDs(DefaultContext, dummyCanary.ID.String())
			Expect(err).To(BeNil())
			Expect(ids).To(HaveLen(3)) // http, dns, webhook
			Expect(ids).To(ContainElements(
				httpTransformedID.String(),
				dnsTransformedID.String(),
				webhookTransformedID.String(),
			))
		})

		ginkgo.It("should exclude single type using <> operator", func() {
			ids, err := GetTransformedCheckIDs(DefaultContext, dummyCanary.ID.String(), checks.WebhookCheckType)
			Expect(err).To(BeNil())
			Expect(ids).To(HaveLen(2)) // http, dns (webhook excluded)
			Expect(ids).To(ContainElements(
				httpTransformedID.String(),
				dnsTransformedID.String(),
			))
			Expect(ids).ToNot(ContainElement(webhookTransformedID.String()))
		})

		ginkgo.It("should exclude multiple types using IN operator with set difference", func() {
			// Exclude both webhook and dns types
			ids, err := GetTransformedCheckIDs(DefaultContext, dummyCanary.ID.String(), checks.WebhookCheckType, "dns")
			Expect(err).To(BeNil())
			Expect(ids).To(HaveLen(1)) // http only (webhook and dns excluded)
			Expect(ids).To(ContainElement(httpTransformedID.String()))
			Expect(ids).ToNot(ContainElements(webhookTransformedID.String(), dnsTransformedID.String()))
		})

		ginkgo.It("should return empty result when all types are excluded", func() {
			// Exclude all check types present
			ids, err := GetTransformedCheckIDs(DefaultContext, dummyCanary.ID.String(), "http", "dns", checks.WebhookCheckType)
			Expect(err).To(BeNil())
			Expect(ids).To(BeEmpty())
		})

		ginkgo.It("should return empty result for non-existent canary", func() {
			nonExistentID := uuid.New()
			ids, err := GetTransformedCheckIDs(DefaultContext, nonExistentID.String())
			Expect(err).To(BeNil())
			Expect(ids).To(BeEmpty())
		})

		ginkgo.It("should handle excluding non-existent types gracefully", func() {
			ids, err := GetTransformedCheckIDs(DefaultContext, dummyCanary.ID.String(), "nonexistent-type")
			Expect(err).To(BeNil())
			Expect(ids).To(HaveLen(3)) // All transformed checks still returned
		})
	})
})
