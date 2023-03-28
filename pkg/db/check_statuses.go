package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"gorm.io/gorm/clause"
)

const (
	DefaultCheckStatusRetentionDays        = 60
	RetentionDaysFor1hCheckStatusAggregate = 180
	RetentionDaysFor1dCheckStatusAggregate = 1000

	checkStatusAggDuration1h = "1hour"
	checkStatusAggDuration1d = "1day"
)

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

// DeleteOld1dAggregatedCheckStatuses maintains retention period of old 1day aggregate check statuses.
func DeleteOld1dAggregatedCheckStatuses() {
	jobHistory := models.NewJobHistory("DeleteOld1dAggregatedCheckStatuses", "", "").Start()
	if err := PersistJobHistory(jobHistory); err != nil {
		logger.Errorf("error persisting job history: %v", err)
	}
	defer func() {
		if err := PersistJobHistory(jobHistory.End()); err != nil {
			logger.Errorf("error persisting end of job: %v", err)
		}
	}()

	const query = `DELETE FROM check_statuses_1d WHERE (NOW() - created_at) > INTERVAL '1 day' * ?`
	tx := Gorm.Exec(query, RetentionDaysFor1dCheckStatusAggregate)
	if tx.Error != nil {
		logger.Errorf("error deleting old aggregated check statuses: %v", tx.Error)
		jobHistory.AddError(tx.Error.Error())
		return
	}

	logger.Infof("Successfully deleted %v entries", tx.RowsAffected)
	jobHistory.IncrSuccess()
}

// DeleteOld1hAggregatedCheckStatuses maintains retention period of old 1hour aggregated check statuses.
func DeleteOld1hAggregatedCheckStatuses() {
	jobHistory := models.NewJobHistory("DeleteOld1hAggregatedCheckStatuses", "", "").Start()
	if err := PersistJobHistory(jobHistory); err != nil {
		logger.Errorf("error persisting job history: %v", err)
	}
	defer func() {
		if err := PersistJobHistory(jobHistory.End()); err != nil {
			logger.Errorf("error persisting end of job: %v", err)
		}
	}()

	const query = `DELETE FROM check_statuses_1h WHERE (NOW() - created_at) > INTERVAL '1 day' * ?`
	tx := Gorm.Exec(query, RetentionDaysFor1hCheckStatusAggregate)
	if tx.Error != nil {
		logger.Errorf("error deleting old aggregated check statuses: %v", tx.Error)
		jobHistory.AddError(tx.Error.Error())
		return
	}

	logger.Infof("Successfully deleted %v entries", tx.RowsAffected)
	jobHistory.IncrSuccess()
}

// AggregateCheckStatuses aggregates check statuses hourly
// and stores it to the corresponding tables.
func AggregateCheckStatuses1h() {
	jobHistory := models.NewJobHistory("AggregateCheckStatuses1h", "", "").Start()
	if err := PersistJobHistory(jobHistory); err != nil {
		logger.Errorf("error persisting job history: %v", err)
	}
	defer func() {
		if err := PersistJobHistory(jobHistory.End()); err != nil {
			logger.Errorf("error persisting end of job: %v", err)
		}
	}()

	if err := aggregateCheckStatuses(checkStatusAggDuration1h); err != nil {
		logger.Errorf("error aggregating check statuses 1h: %v", err)
		jobHistory.AddError(err.Error())
	} else {
		jobHistory.IncrSuccess()
	}
}

// AggregateCheckStatuses aggregates check statuses daily
// and stores it to the corresponding tables.
func AggregateCheckStatuses1d() {
	jobHistory := models.NewJobHistory("AggregateCheckStatuses1d", "", "").Start()
	if err := PersistJobHistory(jobHistory); err != nil {
		logger.Errorf("error persisting job history: %v", err)
	}
	defer func() {
		if err := PersistJobHistory(jobHistory.End()); err != nil {
			logger.Errorf("error persisting end of job: %v", err)
		}
	}()

	if err := aggregateCheckStatuses(checkStatusAggDuration1d); err != nil {
		logger.Errorf("error aggregating check statuses 1d: %v", err)
		jobHistory.AddError(err.Error())
	} else {
		jobHistory.IncrSuccess()
	}
}

func aggregateCheckStatuses(aggregateDurationType string) error {
	const query = `
		SELECT
		check_statuses.check_id,
		date_trunc(?, "time"),
		count(*) AS total_checks,
		count(*) FILTER (WHERE check_statuses.status = TRUE) AS passed,
		count(*) FILTER (WHERE check_statuses.status = FALSE) AS failed,
		SUM(duration) AS duration
	FROM check_statuses 
	LEFT JOIN checks ON check_statuses.check_id = checks.id
	WHERE checks.created_at > NOW() - INTERVAL '1 day' * ?
	GROUP BY 1, 2
	ORDER BY 1,	2 DESC`

	var rows *sql.Rows
	var err error
	switch aggregateDurationType {
	case checkStatusAggDuration1h:
		rows, err = Gorm.Raw(query, "hour", 1).Rows() // Only look for aggregated data in the last day
		if err != nil {
			return fmt.Errorf("error aggregating check statuses 1h: %w", err)
		} else if rows.Err() != nil {
			return fmt.Errorf("error aggregating check statuses 1h: %w", rows.Err())
		}
		defer rows.Close()

		for rows.Next() {
			var aggr models.CheckStatusAggregate1h
			if err := rows.Scan(&aggr.CheckID, &aggr.CreatedAt, &aggr.Total, &aggr.Passed, &aggr.Failed, &aggr.Duration); err != nil {
				return fmt.Errorf("error scanning aggregated check statuses: %w", err)
			}

			cols := []clause.Column{{Name: "check_id"}, {Name: "created_at"}}
			if err := Gorm.Clauses(clause.OnConflict{Columns: cols, UpdateAll: true}).Create(aggr).Error; err != nil {
				return fmt.Errorf("error upserting canaries: %w", err)
			}
		}

	case checkStatusAggDuration1d:
		rows, err = Gorm.Raw(query, "day", 7).Rows() // Only look for aggregated data in the last 7 days
		if err != nil {
			return fmt.Errorf("error aggregating check statuses 1h: %w", err)
		} else if rows.Err() != nil {
			return fmt.Errorf("error aggregating check statuses 1h: %w", rows.Err())
		}
		defer rows.Close()

		for rows.Next() {
			var aggr models.CheckStatusAggregate1d
			if err := rows.Scan(&aggr.CheckID, &aggr.CreatedAt, &aggr.Total, &aggr.Passed, &aggr.Failed, &aggr.Duration); err != nil {
				return fmt.Errorf("error scanning aggregated check statuses: %w", err)
			}

			cols := []clause.Column{{Name: "check_id"}, {Name: "created_at"}}
			if err := Gorm.Clauses(clause.OnConflict{Columns: cols, UpdateAll: true}).Create(aggr).Error; err != nil {
				return fmt.Errorf("error upserting canaries: %w", err)
			}
		}

	default:
		return errors.New("unknown duration for aggregation")
	}

	return nil
}
