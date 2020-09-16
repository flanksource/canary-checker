package cache

import (
	"sort"
	"sync"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
)

var Size = 5

type cache struct {
	Checks       map[string]pkg.Check
	CheckConfigs map[string]external.Check
	mtx          sync.Mutex
}

var Cache = &cache{
	Checks:       make(map[string]pkg.Check),
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

func GetChecks() pkg.Checks {
	return Cache.GetChecks()
}

func (c *cache) RemoveCheck(checks v1.Canary) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, check := range checks.Spec.GetAllChecks() {
		key := checks.GetKey(check)
		logger.Errorf("removing %s", key)
		delete(c.Checks, key)
		delete(c.CheckConfigs, key)
	}
}

func (c *cache) InitCheck(checks v1.Canary) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	// initialize all checks so that they appear on the dashboard as pending
	for _, check := range checks.Spec.GetAllChecks() {
		key := checks.GetKey(check)
		c.Checks[key] = pkg.Check{
			Type:        check.GetType(),
			Name:        checks.ID(),
			Description: check.GetDescription(),
			Endpoint:    check.GetEndpoint(),
		}
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

	check := pkg.Check{
		Key:         checks.GetKey(result.Check),
		Type:        result.Check.GetType(),
		Name:        checks.ID(),
		Description: checks.GetDescription(result.Check),
		Endpoint:    result.Check.GetEndpoint(),
		CheckCanary: &checks,
		Statuses: []pkg.CheckStatus{
			{
				Status:   result.Pass,
				Invalid:  result.Invalid,
				Duration: int(result.Duration),
				Time:     pkg.JSONTime(time.Now().UTC()),
				Message:  result.Message,
			},
		},
	}

	lastCheck, found := c.Checks[check.Key]
	if found {
		check.Statuses = append(check.Statuses, lastCheck.Statuses...)
		if len(check.Statuses) > Size {
			check.Statuses = check.Statuses[:Size]
		}
	} else {
	}
	c.Checks[check.Key] = check
	return &lastCheck
}

func (s *cache) GetChecks() pkg.Checks {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	result := pkg.Checks{}

	for _, check := range s.Checks {
		uptime, latency := metrics.GetMetrics(check.Key)
		check.Uptime = uptime
		check.Latency = latency.String()
		result = append(result, check)
	}

	sort.Sort(&result)
	return result
}
