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
	DefaultCheckStatusRetentionDays = 60

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
		count(*) FILTER (WHERE check_statuses.status = TRUE) AS successful_checks,
		count(*) FILTER (WHERE check_statuses.status = FALSE) AS failed_checks,
		SUM(duration) AS total_duration
	FROM check_statuses 
	LEFT JOIN checks ON check_statuses.check_id = checks.id
	WHERE checks.created_at > NOW() - INTERVAL '7 day'
	GROUP BY 1, 2
	ORDER BY 1,	2 DESC`

	var rows *sql.Rows
	var err error
	switch aggregateDurationType {
	case checkStatusAggDuration1h:
		rows, err = Gorm.Raw(query, "hour").Rows()
		if err != nil {
			return fmt.Errorf("error aggregating check statuses 1h: %w", err)
		}
		defer rows.Close()

	case checkStatusAggDuration1d:
		rows, err = Gorm.Raw(query, "day").Rows()
		if err != nil {
			return fmt.Errorf("error aggregating check statuses 1h: %w", err)
		}
		defer rows.Close()

	default:
		return errors.New("unknown duration for aggregation")
	}

	for rows.Next() {
		var aggr models.CheckStatusAggregate
		aggr.IntervalDuration = aggregateDurationType
		if err := rows.Scan(&aggr.CheckID, &aggr.IntervalStart, &aggr.TotalChecks, &aggr.SuccessfulChecks, &aggr.FailedChecks, &aggr.TotalDuration); err != nil {
			return fmt.Errorf("error scanning aggregated check statuses: %w", err)
		}

		cols := []clause.Column{{Name: "check_id"}, {Name: "created_at"}, {Name: "interval_duration"}}
		if err := Gorm.Clauses(clause.OnConflict{Columns: cols, UpdateAll: true}).Create(aggr).Error; err != nil {
			return fmt.Errorf("error upserting canaries: %w", err)
		}
	}

	return nil
}
