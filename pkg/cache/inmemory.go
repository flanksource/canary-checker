package cache

import (
	"sync"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
)

type inMemoryCache struct {
	Checks map[string]*pkg.Check `json:"checks"`
	// Key is the "checkKey"-status
	Statuses map[string][]pkg.CheckStatus `json:"status"`
	mtx      sync.Mutex
	// the string is checkKey
	Details map[string][]interface{} `json:"details"`
}

var InMemoryCache = &inMemoryCache{
	Checks:   make(map[string]*pkg.Check),
	Statuses: make(map[string][]pkg.CheckStatus),
}

func (c *inMemoryCache) InitCheck(canary v1.Canary) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	// initialize all checks so that they appear on the dashboard as pending
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		pkgCheck := pkg.FromV1(canary, check)
		c.Checks[key] = &pkgCheck
		c.Statuses[key] = []pkg.CheckStatus{}
	}
}

func (c *inMemoryCache) Add(check pkg.Check, status pkg.CheckStatus) {
	check.Statuses = nil
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.AddCheck(check)
	if err := c.AppendCheckStatus(check.Key, status); err != nil {
		logger.Debugf("error appending check status: %v", err)
	}
}

func (c *inMemoryCache) AddCheck(check pkg.Check) {
	c.Checks[check.Key] = &check
}

func (c *inMemoryCache) AppendCheckStatus(checkKey string, checkStatus pkg.CheckStatus) error {
	c.Statuses[checkKey] = append([]pkg.CheckStatus{checkStatus}, c.Statuses[checkKey]...)
	if len(c.Statuses[checkKey]) > InMemoryCacheSize {
		c.Statuses[checkKey] = c.Statuses[(checkKey)][:InMemoryCacheSize]
	}
	return nil
}

func (c *inMemoryCache) GetChecks() pkg.Checks {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	result := pkg.Checks{}
	for _, check := range c.Checks {
		uptime, latency := metrics.GetMetrics(check.Key)
		check.Uptime = uptime
		check.Latency = latency
		result = append(result, check)
	}
	return result
}

func (c *inMemoryCache) GetCheckFromKey(checkkey string) *pkg.Check {
	return c.Checks[checkkey]
}

func (c *inMemoryCache) GetCheckFromID(id string) *pkg.Check {
	for _, check := range c.Checks {
		if check.ID == id {
			return check
		}
	}
	return nil
}

// GetDetails returns the details for a given check
func (c *inMemoryCache) GetDetails(checkkey string, time string) interface{} {
	if statuses, ok := c.Statuses[checkkey]; ok {
		for _, status := range statuses {
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

func (c *inMemoryCache) ListCheckStatus(checkKey string, count int64, duration *time.Duration) []pkg.CheckStatus {
	var result []pkg.CheckStatus
	if duration != nil {
		startTime := time.Now().UTC()
		var i int64 = 0
		for _, status := range c.Statuses[checkKey] {
			if i <= count && count != AllStatuses {
				break
			}
			checkTime, err := time.Parse(time.RFC3339, status.Time)
			if err != nil {
				logger.Errorf("error parsing time: %v", err)
				continue
			}
			if checkTime.Add(*duration).Before(startTime) {
				return result
			}
			result = append(result, status)
			i += 1
		}
	}
	statuses := c.Statuses[checkKey]
	if len(statuses) < int(count) || count == AllStatuses {
		return statuses
	}
	return statuses[0 : count-1]
}

func (c *inMemoryCache) RemoveChecks(canary v1.Canary) {
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		logger.Debugf("removing %s", key)
		c.RemoveCheckByKey(key)
	}
}

func (c *inMemoryCache) RemoveCheckByKey(key string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	delete(c.Checks, key)
	delete(c.Statuses, key)
}
