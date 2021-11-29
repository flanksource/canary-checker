package cache

import (
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type cacheChain struct {
	Chain []Cache
}

var CacheChain = &cacheChain{
	Chain: []Cache{
		InMemoryCache,
	},
}

func (c *cacheChain) Add(check pkg.Check, result pkg.CheckStatus) {
	for _, cache := range c.Chain {
		cache.Add(check, result)
	}
}

func (c *cacheChain) GetChecks() pkg.Checks {
	var checksMap = make(map[string]bool)
	var checks pkg.Checks
	for _, cache := range c.Chain {
		cacheChecks := cache.GetChecks()
		for _, check := range cacheChecks {
			if _, ok := checksMap[check.Key]; !ok {
				checksMap[check.Key] = true
				checks = append(checks, check)
			}
		}
	}
	return checks
}

func (c *cacheChain) GetCheckFromKey(key string) *pkg.Check {
	var check *pkg.Check
	for _, cache := range c.Chain {
		check = cache.GetCheckFromKey(key)
		if check != nil {
			return check
		}
	}
	return check
}

func (c *cacheChain) GetDetails(checkkey string, time string) interface{} {
	for i, cache := range c.Chain {
		details := cache.GetDetails(checkkey, time)
		if details != nil {
			fmt.Printf("sending the detail back from: %vth cache ", i)
			return details
		}
	}
	return nil
}

func (c *cacheChain) ListCheckStatus(checkKey string, count int64, duration *time.Duration) []pkg.CheckStatus {
	var statuses []pkg.CheckStatus
	var statusMap = make(map[string]bool)
	for _, cache := range c.Chain {
		checkStatuses := cache.ListCheckStatus(checkKey, count, duration)
		for _, status := range checkStatuses {
			if _, ok := statusMap[status.Time]; !ok {
				statuses = append(statuses, status)
				statusMap[status.Time] = true
			}
		}
	}
	return statuses
}

func (c *cacheChain) RemoveChecks(canary v1.Canary) {
	for _, cache := range c.Chain {
		cache.RemoveChecks(canary)
	}
}

func (c *cacheChain) RemoveCheckByKey(key string) {
	for _, cache := range c.Chain {
		cache.RemoveCheckByKey(key)
	}
}
