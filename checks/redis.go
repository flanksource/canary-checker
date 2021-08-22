package checks

import (
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/kommons"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/go-redis/redis/v8"
)

func init() {
	//register metrics here
}

type RedisChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *RedisChecker) SetClient(client *kommons.Client) {
	c.kommons = client
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
	start := time.Now()
	redisCheck := extConfig.(v1.RedisCheck)
	namespace := ctx.Canary.Namespace
	var err error
	auth, err := GetAuthValues(redisCheck.Auth, c.kommons, namespace)
	if err != nil {
		return Failf(redisCheck, "failed to fetch auth details: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisCheck.Addr,
		Password: auth.GetPassword(),
		DB:       redisCheck.DB,
		Username: auth.GetUsername(),
	})
	result, err := rdb.Ping(ctx).Result()

	if err != nil {
		return Failf(redisCheck, "failed to execute query %s", err)
	}
	if result != "PONG" {
		return Failf(redisCheck, "expected PONG as result, got %s", result)
	}
	return Success(redisCheck, start)
}
