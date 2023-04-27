package checks

import (
	"strconv"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty"

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
func (c *RedisChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Redis {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *RedisChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.RedisCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	namespace := ctx.Canary.Namespace

	var redisOpts *redis.Options
	if check.ConnectionName != "" {
		connection, err := duty.FindConnectionByURL(ctx, db.Gorm, check.ConnectionName)
		if err != nil {
			return results.Failf("failed to fetch connection: %v", err)
		} else if connection != nil {
			redisOpts = &redis.Options{
				Addr:     connection.URL,
				Username: connection.Username,
				Password: connection.Password,
			}

			if db, ok := connection.Properties["db"]; ok {
				if dbInt, err := strconv.Atoi(db); nil == err {
					redisOpts.DB = dbInt
				}
			}
		}
	} else {
		auth, err := GetAuthValues(check.Auth, ctx.Kommons, namespace)
		if err != nil {
			return results.Failf("failed to fetch auth details: %v", err)
		}

		redisOpts = &redis.Options{
			Addr: check.Addr,
			DB:   check.DB,
		}
		if auth != nil {
			redisOpts.Username = auth.GetUsername()
			redisOpts.Password = auth.GetPassword()
		}
	}

	rdb := redis.NewClient(redisOpts)
	queryResult, err := rdb.Ping(ctx).Result()
	if err != nil {
		return results.Failf("failed to execute query %v", err)
	}

	if queryResult != "PONG" {
		return results.Failf("expected PONG as result, got %s", result)
	}

	return results
}
