package cache

import (
	"fmt"

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

func (c *cacheChain) QueryStatus(q QueryParams) ([]pkg.Timeseries, error) {
	return nil, nil
}

func (c *cacheChain) Query(q QueryParams) (pkg.Checks, error) {
	checks := pkg.Checks{}
	for _, cache := range c.Chain {
		results, err := cache.Query(q)
		if err != nil {
			return nil, err
		}
		checks = checks.Merge(results)
	}
	return checks, nil
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
