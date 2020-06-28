package cache

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var Size = 5

type cache struct {
	Checks map[string]pkg.Check
	mtx    sync.Mutex
}

var Cache = &cache{
	Checks: make(map[string]pkg.Check),
}

func AddCheck(result *pkg.CheckResult) {
	Cache.AddCheck(result)
}

func GetChecks() pkg.Checks {
	return Cache.GetChecks()
}

func (c *cache) AddCheck(result *pkg.CheckResult) {
	if result == nil || result.Check == nil {
		logger.Warnf("result with no check found: %+v", result)
		return
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()

	check := pkg.Check{
		Type: result.Check.GetType(),
		Name: result.Check.GetEndpoint(),
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

	key := fmt.Sprintf("%s/%s", check.Type, check.Name)

	lastCheck, found := c.Checks[key]
	if found {
		check.Statuses = append(check.Statuses, lastCheck.Statuses...)
		if len(check.Statuses) > Size {
			check.Statuses = check.Statuses[:Size]
		}
	}
	c.Checks[key] = check
}

func (s *cache) GetChecks() pkg.Checks {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	result := pkg.Checks{}

	for _, m := range s.Checks {
		result = append(result, m)
	}

	sort.Sort(&result)
	return result
}
