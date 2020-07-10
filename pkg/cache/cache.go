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

func AddCheck(name string, result *pkg.CheckResult) *pkg.Check {
	return Cache.AddCheck(name, result)
}

func GetChecks() pkg.Checks {
	return Cache.GetChecks()
}

func (c *cache) AddCheck(name string, result *pkg.CheckResult) *pkg.Check {
	if result == nil || result.Check == nil {
		logger.Warnf("result with no check found: %+v", result)
		return nil
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()

	description := result.Check.GetDescription()
	if description == "" {
		description = result.Check.GetEndpoint()
	}
	check := pkg.Check{
		Type:        result.Check.GetType(),
		Name:        name,
		Description: description,
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
	} else {
	}
	c.Checks[key] = check
	return &lastCheck
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
