package checks

import (
	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/go-redis/redis/v8"
)

func init() {
	//register metrics here
}

type RedisChecker struct {
}

// Type: returns checker type
func (c *RedisChecker) Type() string {
	return "redis"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *RedisChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Redis {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

func (c *RedisChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	updated, err := Contextualise(extConfig, ctx)
	if err != nil {
		return pkg.Fail(extConfig, ctx.Canary)
	}
	redisCheck := updated.(v1.RedisCheck)
	result := pkg.Success(redisCheck, ctx.Canary)
	namespace := ctx.Canary.Namespace
	auth, err := GetAuthValues(redisCheck.Auth, ctx.Kommons, namespace)
	if err != nil {
		return result.Failf("failed to fetch auth details: %v", err)
	}
	opts := &redis.Options{
		Addr: redisCheck.Addr,
		DB:   redisCheck.DB,
	}
	if auth != nil {
		opts.Username = auth.GetUsername()
		opts.Password = auth.GetPassword()
	}

	rdb := redis.NewClient(opts)
	results, err := rdb.Ping(ctx).Result()

	if err != nil {
		return result.Failf("failed to execute query %s", err)
	}
	if results != "PONG" {
		return result.Failf("expected PONG as result, got %s", result)
	}
	return result
}
