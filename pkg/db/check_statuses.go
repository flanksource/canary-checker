package db

import "github.com/flanksource/commons/logger"

const DefaultCheckStatusRetentionDays = 60

var CheckStatusRetentionDays int

func DeleteOldCheckStatuses() {
	if CheckStatusRetentionDays <= 0 {
		CheckStatusRetentionDays = DefaultCheckStatusRetentionDays
	}
	err := Gorm.Exec(`
        DELETE FROM check_statuses
        WHERE (NOW() - created_at) > INTERVAL '1 day' * ?
    `, CheckStatusRetentionDays).Error

	if err != nil {
		logger.Errorf("Error deleting old check statuses: %v", err)
	}
}
