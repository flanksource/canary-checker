package db

import (
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
)

func PersistJobHistory(h *models.JobHistory) error {
	if Gorm == nil {
		return nil
	}

	// Delete jobs which did not process anything
	if h.ID != uuid.Nil && (h.SuccessCount+h.ErrorCount) == 0 {
		return Gorm.Table("job_history").Delete(h).Error
	}

	return Gorm.Table("job_history").Save(h).Error
}
