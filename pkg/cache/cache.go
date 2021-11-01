package cache

import (
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var Size = 5

type Cache interface {
	Add(check pkg.Check, status pkg.CheckStatus)
	GetChecks() pkg.Checks
	GetCheckFromKey(key string) *pkg.Check
	GetDetails(checkkey string, time string) interface{}
	ListCheckStatus(checkKey string, count int64, duration *time.Duration) []pkg.CheckStatus
	RemoveChecks(canary v1.Canary)
	RemoveCheckByKey(key string)
}

func QueryChecks(cache Cache, count int64, duration *time.Duration, checkKey string) pkg.Checks {
	var checks pkg.Checks
	if checkKey != "" {
		checks = pkg.Checks{cache.GetCheckFromKey(checkKey)}
	} else {
		checks = cache.GetChecks()
	}
	for _, check := range checks {
		check.Statuses = cache.ListCheckStatus(check.Key, count, duration)
		if len(check.Statuses) >= int(count) {
			continue
		}
	}
	return checks
}
