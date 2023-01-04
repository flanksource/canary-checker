package v1

const (
	PostgresTimestampFormat            = "2006-01-02T15:04:05.999"
	SyncCanaryJobsSchedule             = "@every 2m"
	SyncSystemsJobsSchedule            = "@every 5m"
	ComponentRunSchedule               = "@every 2m"
	ComponentStatusSummarySyncSchedule = "@every 1m"
	ComponentCheckSchedule             = "@every 2m"
	ComponentConfigSchedule            = "@every 2m"
	ComponentCostSchedule              = "@every 1h"
)
