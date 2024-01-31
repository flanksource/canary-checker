package db

import (
	"time"

	"github.com/flanksource/duty/job"
)

const CheckStatuses = "check_statuses"

var RefreshCheckStatusSummary = job.Job{
	Name:       "RefreshCheckStatusSummary",
	Singleton:  true,
	Timeout:    1 * time.Minute,
	Schedule:   "@every 1m",
	JobHistory: true,
	Retention: job.Retention{
		Interval: 5 * time.Minute,
		Success:  1,
		Failed:   3,
		Age:      time.Hour * 24,
	},
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		return job.RefreshCheckStatusSummary(ctx.Context)
	},
}

var RefreshCheckStatusSummaryAged = job.Job{
	Name:       "RefreshCheckStatusSummaryAged",
	Timeout:    60 * time.Minute,
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		return job.RefreshCheckStatusSummaryAged(ctx.Context)
	},
}

var DeleteOldCheckStatues = job.Job{
	Name:      "DeleteOldCheckStatuses",
	Singleton: true,
	Schedule:  "@every 24h",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		err, count := job.DeleteOldCheckStatuses1h(ctx.Context, ctx.Properties().Int("check.status.retention.days", 30))
		ctx.History.SuccessCount = count
		return err
	},
}

var DeleteOldCheckStatues1d = job.Job{
	Name:      "DeleteOldCheckStatuses1d",
	Singleton: true,
	Schedule:  "@every 24h",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		err, count := job.DeleteOldCheckStatuses1d(ctx.Context, ctx.Properties().Int("check.status.retention.days", 30)*9)
		ctx.History.SuccessCount = count
		return err
	},
}

var DeleteOldCheckStatues1h = job.Job{
	Name:      "DeleteOldCheckStatuses1h",
	Singleton: true,

	Schedule: "@every 24h",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		err, count := job.DeleteOldCheckStatuses1h(ctx.Context, ctx.Properties().Int("check.status.retention.days", 30)*3)
		ctx.History.SuccessCount = count
		return err
	},
}

var AggregateCheckStatues1d = job.Job{
	Name:     "AggregateCheckStatuses1h",
	Schedule: "@every 1h",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		err, count := job.AggregateCheckStatus1h(ctx.Context)
		ctx.History.SuccessCount = count
		return err
	},
}

var AggregateCheckStatues1h = job.Job{
	Name:     "AggregateCheckStatuses1d",
	Schedule: "@every 24h",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = CheckStatuses
		err, count := job.AggregateCheckStatus1d(ctx.Context)
		ctx.History.SuccessCount = count
		return err
	},
}

var CheckStatusJobs = []job.Job{
	AggregateCheckStatues1d,
	AggregateCheckStatues1h,
	DeleteOldCheckStatues,
	DeleteOldCheckStatues1h,
	DeleteOldCheckStatues1d,
	RefreshCheckStatusSummary,
	RefreshCheckStatusSummaryAged,
}
