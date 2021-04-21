package checks

import (
	"context"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/go-redis/redis/v8"
)

func init() {
	//register metrics here
}

type RedisChecker struct{}

// Type: returns checker type
func (c *RedisChecker) Type() string {
	return "redis"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *RedisChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Redis {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *RedisChecker) Check(extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	redisCheck := extConfig.(v1.RedisCheck)
	result, err := connectRedis(redisCheck.Addr, redisCheck.Password, redisCheck.Username, redisCheck.DB)
	if err != nil {
		return Failf(redisCheck, "failed to execute query %s", err)
	}
	if result != "PONG" {
		return Failf(redisCheck, "expected PONG as result, got %s", result)
	}
	return Success(redisCheck, start)
}

func connectRedis(addr, password, username string, db int) (string, error) {
	ctx := context.TODO()
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		Username: username,
	})
	return rdb.Ping(ctx).Result()
}
