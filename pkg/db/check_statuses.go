package db

import (
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
)

const DefaultCheckStatusRetentionDays = 60

var CheckStatusRetentionDays int

func DeleteOldCheckStatuses() {
	jobHistory := models.NewJobHistory("DeleteOldCheckStatuses", "", "").Start()
	_ = PersistJobHistory(jobHistory)

	if CheckStatusRetentionDays <= 0 {
		CheckStatusRetentionDays = DefaultCheckStatusRetentionDays
	}
	err := Gorm.Exec(`
        DELETE FROM check_statuses
        WHERE (NOW() - created_at) > INTERVAL '1 day' * ?
    `, CheckStatusRetentionDays).Error

	if err != nil {
		logger.Errorf("Error deleting old check statuses: %v", err)
		jobHistory.AddError(err.Error())
	} else {
		jobHistory.IncrSuccess()
	}
	_ = PersistJobHistory(jobHistory.End())
}
