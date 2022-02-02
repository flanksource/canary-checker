package cache

import (
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	cmap "github.com/orcaman/concurrent-map"
)

type inMemoryCache struct {
	Checks cmap.ConcurrentMap `json:"checks"`
	// Key is the "checkKey"-status
	Statuses cmap.ConcurrentMap `json:"status"`
	// the string is checkKey
	Details cmap.ConcurrentMap `json:"details"`
}

var InMemoryCache = &inMemoryCache{
	Checks:   cmap.New(),
	Details:  cmap.New(),
	Statuses: cmap.New(),
}

func (c *inMemoryCache) InitCheck(canary v1.Canary) {
	// initialize all checks so that they appear on the dashboard as pending
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		pkgCheck := pkg.FromV1(canary, check)
		c.Checks.Set(key, &pkgCheck)
		c.Statuses.Set(key, []pkg.CheckStatus{})
	}
}

func (c *inMemoryCache) Add(check pkg.Check, status ...pkg.CheckStatus) {
	check.Statuses = nil
	c.AddCheck(check)
	if err := c.AppendCheckStatus(check.Key, status...); err != nil {
		logger.Debugf("error appending check status: %v", err)
	}
}

func (c *inMemoryCache) AddCheck(check pkg.Check) {
	c.Checks.Set(check.Key, &check)
}

func (c *inMemoryCache) AppendCheckStatus(checkKey string, checkStatus ...pkg.CheckStatus) error {
	val, ok := c.Statuses.Get(checkKey)
	if !ok {
		val = []pkg.CheckStatus{}
	}
	statuses := val.([]pkg.CheckStatus)
	statuses = append(checkStatus, statuses...)
	if len(statuses) > InMemoryCacheSize {
		statuses = statuses[:InMemoryCacheSize]
	}
	c.Statuses.Set(checkKey, statuses)
	return nil
}

func (c *inMemoryCache) GetChecks() pkg.Checks {
	result := pkg.Checks{}
	for v := range c.Checks.IterBuffered() {
		check := v.Val.(*pkg.Check)
		uptime, latency := metrics.GetMetrics(check.Key)
		check.Uptime = uptime
		check.Latency = latency
		result = append(result, check)
	}
	return result
}

func (c *inMemoryCache) GetCheckFromKey(checkkey string) *pkg.Check {
	if v, ok := c.Checks.Get(checkkey); ok {
		return v.(*pkg.Check)
	}
	return nil
}

func (c *inMemoryCache) GetCheckFromID(id string) *pkg.Check {
	for tup := range c.Checks.IterBuffered() {
		check := tup.Val.(*pkg.Check)
		if check.ID == id {
			return check
		}
	}
	return nil
}

// GetDetails returns the details for a given check
func (c *inMemoryCache) GetDetails(checkkey string, time string) interface{} {
	if statuses, ok := c.Statuses.Get(checkkey); ok {
		for _, status := range statuses.([]pkg.CheckStatus) {
			if time == "*" || time == "last" {
				return status.Detail
			}
			if status.Time == time {
				return status.Detail
			}
		}
	}
	return nil
}

func (c *inMemoryCache) QueryStatus(q QueryParams) ([]pkg.Timeseries, error) {
	return nil, nil
}

func (c *inMemoryCache) Query(q QueryParams) (pkg.Checks, error) {
	var checks pkg.Checks
	if q.Check != "" {
		check := c.GetCheckFromKey(q.Check)
		if check == nil {
			return nil, nil
		}
		checks = pkg.Checks{check}
	} else {
		checks = c.GetChecks()
	}
	var results pkg.Checks
	for _, check := range checks {
		if check == nil {
			continue
		}
		check.Statuses = c.ListCheckStatus(check.Key, q)
		if len(check.Statuses) > 0 {
			results = append(results, check)
		}
	}
	return results, nil
}

func (c *inMemoryCache) ListCheckStatus(checkKey string, q QueryParams) []pkg.CheckStatus {
	var result []pkg.CheckStatus
	start := q.GetStartTime()
	end := q.GetEndTime()

	var i int64 = 0
	checks, ok := c.Statuses.Get(checkKey)
	if !ok {
		return nil
	}
	for _, status := range checks.([]pkg.CheckStatus) {
		if i >= int64(q.StatusCount) {
			break
		}
		checkTime, err := time.Parse(time.RFC3339, status.Time)
		if err != nil {
			logger.Errorf("error parsing time: %v", err)
			continue
		}
		if start != nil && checkTime.Before(*start) {
			return result
		}
		if end != nil && checkTime.After(*end) {
			return result
		}
		result = append(result, status)
		i += 1
	}

	return result
}

func (c *inMemoryCache) RemoveChecks(canary v1.Canary) {
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		logger.Debugf("removing %s", key)
		c.RemoveCheckByKey(key)
	}
}

func (c *inMemoryCache) RemoveCheckByKey(key string) {
	c.Checks.Remove(key)
	c.Statuses.Remove(key)
}
