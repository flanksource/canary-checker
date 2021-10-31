package cache

import (
	"sync"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
)

var Size = 5

type cache struct {
	Checks       map[string]*pkg.Check
	CheckConfigs map[string]external.Check
	mtx          sync.Mutex
	// the string is checkKey
	Details map[string][]interface{}
}

var Cache = &cache{
	Checks:       make(map[string]*pkg.Check),
	CheckConfigs: make(map[string]external.Check),
}

func GetConfig(key string) external.Check {
	return Cache.CheckConfigs[key]
}

func AddCheck(check v1.Canary, result *pkg.CheckResult) *pkg.Check {
	return Cache.AddCheck(check, result)
}

func RemoveCheck(checks v1.Canary) {
	Cache.RemoveCheck(checks)
}

func RemoveCheckByKey(key string) {
	Cache.RemoveCheckByKey(key)
}

func GetChecks(duration string) pkg.Checks {
	return Cache.GetChecks(duration)
}

func (c *cache) RemoveCheck(checks v1.Canary) {
	for _, check := range checks.Spec.GetAllChecks() {
		key := checks.GetKey(check)
		logger.Errorf("removing %s", key)
		c.RemoveCheckByKey(key)
	}
}

func (c *cache) RemoveCheckByKey(key string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	delete(c.Checks, key)
}

func (c *cache) InitCheck(canary v1.Canary) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	// initialize all checks so that they appear on the dashboard as pending
	for _, check := range canary.Spec.GetAllChecks() {
		key := canary.GetKey(check)
		pkgCheck := pkg.FromV1(canary, check)
		c.Checks[key] = &pkgCheck
		c.CheckConfigs[key] = check
	}
}

func (c *cache) AddCheck(checks v1.Canary, result *pkg.CheckResult) *pkg.Check {
	if result == nil || result.Check == nil {
		logger.Warnf("result with no check found: %+v", checks.ID())
		return nil
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.Add(pkg.FromV1(checks, result.Check, pkg.FromResult(*result)))
}

func (c *cache) Add(check pkg.Check) *pkg.Check {
	lastCheck, found := c.Checks[check.Key]
	if found {
		check.Statuses = append(check.Statuses, lastCheck.Statuses...)
		if len(check.Statuses) > Size {
			check.Statuses = check.Statuses[:Size]
		}
	}
	c.Checks[check.Key] = &check
	return lastCheck
}

func (c *cache) GetChecks(duration string) pkg.Checks {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	result := pkg.Checks{}
	for _, check := range c.Checks {
		uptime, latency := metrics.GetMetrics(check.Key)
		check.Uptime = uptime
		check.Latency = latency
		if duration != "" {
			metrics.FillLatencies(check.Key, duration, &check.Latency) //nolint: errcheck
			metrics.FillUptime(check.Key, duration, &check.Uptime)     //nolint: errcheck
		}
		result = append(result, check)
	}
	return result
}

func (c *cache) GetCheckFromKey(checkkey string) *pkg.Check {
	return c.Checks[checkkey]
}

func (c *cache) GetCheckFromID(id string) *pkg.Check {
	for _, check := range c.Checks {
		if check.ID == id {
			return check
		}
	}
	return nil
}

// GetDetails returns the details for a given check
func (c *cache) GetDetails(checkkey string, time string) interface{} {
	statuses := c.Checks[checkkey].Statuses
	for _, status := range statuses {
		if status.Time == time {
			return status.Detail
		}
	}
	return nil
}
