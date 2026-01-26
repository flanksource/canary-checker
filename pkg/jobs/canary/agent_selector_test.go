// ABOUTME: Tests for the AgentSelector feature that creates derived canaries for each matched agent.
// ABOUTME: Validates creation, update, and cleanup of derived canaries based on agentSelector field.

package canary

import (
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("AgentSelector", ginkgo.Ordered, func() {
	var testAgent1 models.Agent
	var testAgent2 models.Agent
	var testAgentTeamA models.Agent
	var testAgentTeamB models.Agent
	var testAgentOther models.Agent
	var parentCanaryID uuid.UUID

	ginkgo.BeforeAll(func() {
		testAgent1 = models.Agent{ID: uuid.New(), Name: "test-agent-selector-1"}
		err := DefaultContext.DB().Create(&testAgent1).Error
		Expect(err).To(BeNil())

		testAgent2 = models.Agent{ID: uuid.New(), Name: "test-agent-selector-2"}
		err = DefaultContext.DB().Create(&testAgent2).Error
		Expect(err).To(BeNil())

		testAgentTeamA = models.Agent{ID: uuid.New(), Name: "team-a"}
		err = DefaultContext.DB().Create(&testAgentTeamA).Error
		Expect(err).To(BeNil())

		testAgentTeamB = models.Agent{ID: uuid.New(), Name: "team-b"}
		err = DefaultContext.DB().Create(&testAgentTeamB).Error
		Expect(err).To(BeNil())

		testAgentOther = models.Agent{ID: uuid.New(), Name: "other-agent"}
		err = DefaultContext.DB().Create(&testAgentOther).Error
		Expect(err).To(BeNil())
	})

	ginkgo.AfterAll(func() {
		// Cleanup canaries first due to foreign key constraint
		DefaultContext.DB().Exec("DELETE FROM canaries WHERE name LIKE 'agent-selector-test%'")

		// cleanup agents
		DefaultContext.DB().Delete(&testAgent1)
		DefaultContext.DB().Delete(&testAgent2)
		DefaultContext.DB().Delete(&testAgentTeamA)
		DefaultContext.DB().Delete(&testAgentTeamB)
		DefaultContext.DB().Delete(&testAgentOther)
	})

	ginkgo.Describe("SyncAgentSelectorCanaries", func() {
		ginkgo.It("should create derived canaries for each agent in selector", func() {
			parentCanaryID = uuid.New()

			canarySpec := v1.CanarySpec{
				Schedule: "@every 5m",
				HTTP: []v1.HTTPCheck{
					{
						Connection: v1.Connection{URL: "https://example.com"},
					},
				},
				AgentSelector: []string{testAgent1.Name, testAgent2.Name},
			}

			specBytes, err := json.Marshal(canarySpec)
			Expect(err).To(BeNil())

			var spec types.JSON
			err = json.Unmarshal(specBytes, &spec)
			Expect(err).To(BeNil())

			parentCanary := &models.Canary{
				ID:        parentCanaryID,
				Name:      "agent-selector-test-canary",
				Namespace: "default",
				Spec:      spec,
			}
			err = DefaultContext.DB().Create(parentCanary).Error
			Expect(err).To(BeNil())

			// Run the sync job
			SyncAgentSelectorCanaries.Context = DefaultContext
			SyncAgentSelectorCanaries.Run()

			// Verify derived canaries were created
			var derivedCanaries []models.Canary
			err = DefaultContext.DB().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", parentCanaryID.String())).
				Where("deleted_at IS NULL").
				Find(&derivedCanaries).Error
			Expect(err).To(BeNil())
			Expect(derivedCanaries).To(HaveLen(2))

			// Verify each derived canary has the correct agent_id
			agentIDs := make(map[uuid.UUID]bool)
			for _, c := range derivedCanaries {
				agentIDs[c.AgentID] = true
				Expect(c.Name).To(Equal("agent-selector-test-canary"))
				Expect(c.Namespace).To(Equal("default"))
			}
			Expect(agentIDs).To(HaveKey(testAgent1.ID))
			Expect(agentIDs).To(HaveKey(testAgent2.ID))
		})

		ginkgo.It("should delete derived canaries when agent removed from selector", func() {
			// Update parent canary to remove testAgent2 from selector
			canarySpec := v1.CanarySpec{
				Schedule: "@every 5m",
				HTTP: []v1.HTTPCheck{
					{
						Connection: v1.Connection{URL: "https://example.com"},
					},
				},
				AgentSelector: []string{testAgent1.Name}, // Only agent1 now
			}

			specBytes, err := json.Marshal(canarySpec)
			Expect(err).To(BeNil())

			var spec types.JSON
			err = json.Unmarshal(specBytes, &spec)
			Expect(err).To(BeNil())

			err = DefaultContext.DB().Model(&models.Canary{}).
				Where("id = ?", parentCanaryID).
				Update("spec", spec).Error
			Expect(err).To(BeNil())

			// Run the sync job again
			SyncAgentSelectorCanaries.Run()

			// Verify only one derived canary remains active
			var derivedCanaries []models.Canary
			err = DefaultContext.DB().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", parentCanaryID.String())).
				Where("deleted_at IS NULL").
				Find(&derivedCanaries).Error
			Expect(err).To(BeNil())
			Expect(derivedCanaries).To(HaveLen(1))
			Expect(derivedCanaries[0].AgentID).To(Equal(testAgent1.ID))

			// Verify the other canary was soft-deleted
			var deletedCanary models.Canary
			err = DefaultContext.DB().Unscoped().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", parentCanaryID.String())).
				Where("agent_id = ?", testAgent2.ID).
				First(&deletedCanary).Error
			Expect(err).To(BeNil())
			Expect(deletedCanary.DeletedAt).ToNot(BeNil())
		})

		ginkgo.It("should select agents using glob pattern", func() {
			globCanaryID := uuid.New()

			canarySpec := v1.CanarySpec{
				Schedule: "@every 5m",
				HTTP: []v1.HTTPCheck{
					{
						Connection: v1.Connection{URL: "https://example.com"},
					},
				},
				AgentSelector: []string{"team-*"},
			}

			specBytes, err := json.Marshal(canarySpec)
			Expect(err).To(BeNil())

			var spec types.JSON
			err = json.Unmarshal(specBytes, &spec)
			Expect(err).To(BeNil())

			globCanary := &models.Canary{
				ID:        globCanaryID,
				Name:      "agent-selector-test-glob",
				Namespace: "default",
				Spec:      spec,
			}
			err = DefaultContext.DB().Create(globCanary).Error
			Expect(err).To(BeNil())

			// Run the sync job
			SyncAgentSelectorCanaries.Run()

			// Verify derived canaries were created for team-a and team-b
			var derivedCanaries []models.Canary
			err = DefaultContext.DB().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", globCanaryID.String())).
				Where("deleted_at IS NULL").
				Find(&derivedCanaries).Error
			Expect(err).To(BeNil())
			Expect(derivedCanaries).To(HaveLen(2))

			// Verify each derived canary has the correct agent_id
			agentIDs := make(map[uuid.UUID]bool)
			for _, c := range derivedCanaries {
				agentIDs[c.AgentID] = true
			}
			Expect(agentIDs).To(HaveKey(testAgentTeamA.ID))
			Expect(agentIDs).To(HaveKey(testAgentTeamB.ID))
			Expect(agentIDs).ToNot(HaveKey(testAgentOther.ID))
		})

		ginkgo.It("should exclude agents using negation pattern", func() {
			excludeCanaryID := uuid.New()

			canarySpec := v1.CanarySpec{
				Schedule: "@every 5m",
				HTTP: []v1.HTTPCheck{
					{
						Connection: v1.Connection{URL: "https://example.com"},
					},
				},
				AgentSelector: []string{"team-*", "!team-b"},
			}

			specBytes, err := json.Marshal(canarySpec)
			Expect(err).To(BeNil())

			var spec types.JSON
			err = json.Unmarshal(specBytes, &spec)
			Expect(err).To(BeNil())

			excludeCanary := &models.Canary{
				ID:        excludeCanaryID,
				Name:      "agent-selector-test-exclude",
				Namespace: "default",
				Spec:      spec,
			}
			err = DefaultContext.DB().Create(excludeCanary).Error
			Expect(err).To(BeNil())

			// Run the sync job
			SyncAgentSelectorCanaries.Run()

			// Verify only team-a was selected (team-b excluded)
			var derivedCanaries []models.Canary
			err = DefaultContext.DB().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", excludeCanaryID.String())).
				Where("deleted_at IS NULL").
				Find(&derivedCanaries).Error
			Expect(err).To(BeNil())
			Expect(derivedCanaries).To(HaveLen(1))
			Expect(derivedCanaries[0].AgentID).To(Equal(testAgentTeamA.ID))
		})

		ginkgo.It("should select all agents except excluded ones when only exclusion patterns", func() {
			excludeOnlyCanaryID := uuid.New()

			canarySpec := v1.CanarySpec{
				Schedule: "@every 5m",
				HTTP: []v1.HTTPCheck{
					{
						Connection: v1.Connection{URL: "https://example.com"},
					},
				},
				AgentSelector: []string{"!team-b"},
			}

			specBytes, err := json.Marshal(canarySpec)
			Expect(err).To(BeNil())

			var spec types.JSON
			err = json.Unmarshal(specBytes, &spec)
			Expect(err).To(BeNil())

			excludeOnlyCanary := &models.Canary{
				ID:        excludeOnlyCanaryID,
				Name:      "agent-selector-test-exclude-only",
				Namespace: "default",
				Spec:      spec,
			}
			err = DefaultContext.DB().Create(excludeOnlyCanary).Error
			Expect(err).To(BeNil())

			// Run the sync job
			SyncAgentSelectorCanaries.Run()

			// Verify all agents except team-b were selected
			var derivedCanaries []models.Canary
			err = DefaultContext.DB().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", excludeOnlyCanaryID.String())).
				Where("deleted_at IS NULL").
				Find(&derivedCanaries).Error
			Expect(err).To(BeNil())

			agentIDs := make(map[uuid.UUID]bool)
			for _, c := range derivedCanaries {
				agentIDs[c.AgentID] = true
			}
			Expect(agentIDs).To(HaveKey(testAgent1.ID))
			Expect(agentIDs).To(HaveKey(testAgent2.ID))
			Expect(agentIDs).To(HaveKey(testAgentTeamA.ID))
			Expect(agentIDs).To(HaveKey(testAgentOther.ID))
			Expect(agentIDs).ToNot(HaveKey(testAgentTeamB.ID))
		})
	})

	ginkgo.Describe("CleanupOrphanedAgentSelectorCanaries", func() {
		ginkgo.It("should delete derived canaries when parent is deleted", func() {
			orphanParentID := uuid.New()

			// Create a parent canary with agentSelector
			canarySpec := v1.CanarySpec{
				Schedule: "@every 5m",
				HTTP: []v1.HTTPCheck{
					{
						Connection: v1.Connection{URL: "https://example.com"},
					},
				},
				AgentSelector: []string{testAgent1.Name},
			}

			specBytes, err := json.Marshal(canarySpec)
			Expect(err).To(BeNil())

			var spec types.JSON
			err = json.Unmarshal(specBytes, &spec)
			Expect(err).To(BeNil())

			orphanParent := &models.Canary{
				ID:        orphanParentID,
				Name:      "agent-selector-test-orphan",
				Namespace: "default",
				Spec:      spec,
			}
			err = DefaultContext.DB().Create(orphanParent).Error
			Expect(err).To(BeNil())

			// Run sync to create derived canaries
			SyncAgentSelectorCanaries.Run()

			// Verify derived canary exists
			var derivedCanary models.Canary
			err = DefaultContext.DB().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", orphanParentID.String())).
				Where("deleted_at IS NULL").
				First(&derivedCanary).Error
			Expect(err).To(BeNil())

			// Soft-delete the parent canary
			now := time.Now()
			err = DefaultContext.DB().Model(&models.Canary{}).
				Where("id = ?", orphanParentID).
				Update("deleted_at", now).Error
			Expect(err).To(BeNil())

			// Run cleanup job
			CleanupOrphanedAgentSelectorCanaries.Context = DefaultContext
			CleanupOrphanedAgentSelectorCanaries.Run()

			// Verify derived canary was soft-deleted
			var cleanedCanary models.Canary
			err = DefaultContext.DB().Unscoped().
				Where("source = ?", fmt.Sprintf("agentSelector=%s", orphanParentID.String())).
				First(&cleanedCanary).Error
			Expect(err).To(BeNil())
			Expect(cleanedCanary.DeletedAt).ToNot(BeNil())
		})
	})
})

var _ = ginkgo.Describe("SyncCanaryJob with AgentSelector", ginkgo.Ordered, func() {
	var agentSelectorCanaryID uuid.UUID
	var testAgent models.Agent

	ginkgo.BeforeAll(func() {
		testAgent = models.Agent{ID: uuid.New(), Name: "test-sync-skip-agent"}
		err := DefaultContext.DB().Create(&testAgent).Error
		Expect(err).To(BeNil())
	})

	ginkgo.AfterAll(func() {
		DefaultContext.DB().Delete(&testAgent)
		DefaultContext.DB().Exec("DELETE FROM canaries WHERE id = ?", agentSelectorCanaryID)
	})

	ginkgo.It("should skip canaries with agentSelector in SyncCanaryJob", func() {
		agentSelectorCanaryID = uuid.New()

		canarySpec := v1.CanarySpec{
			Schedule: "@every 1s",
			HTTP: []v1.HTTPCheck{
				{
					Connection: v1.Connection{URL: "https://example.com"},
				},
			},
			AgentSelector: []string{testAgent.Name},
		}

		specBytes, err := json.Marshal(canarySpec)
		Expect(err).To(BeNil())

		var spec types.JSON
		err = json.Unmarshal(specBytes, &spec)
		Expect(err).To(BeNil())

		canary := pkg.Canary{
			ID:        agentSelectorCanaryID,
			Name:      "agent-selector-test-skip",
			Namespace: "default",
			Spec:      spec,
		}

		// Call SyncCanaryJob directly
		err = SyncCanaryJob(DefaultContext, canary)
		Expect(err).To(BeNil())

		// Verify no job was scheduled for this canary
		_, exists := canaryJobs.Load(agentSelectorCanaryID.String())
		Expect(exists).To(BeFalse())
	})
})
