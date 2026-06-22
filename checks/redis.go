package checks

import (
	"crypto/tls"
	"strconv"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/redis/go-redis/v9"
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

	var redisOpts *redis.Options

	//nolint:staticcheck
	if check.Addr != "" && check.URL == "" {
		check.URL = check.Addr
	}

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	redisOpts = &redis.Options{
		Addr:     connection.URL,
		Username: connection.Username,
		Password: connection.Password,
	}

	if check.DB != nil {
		redisOpts.DB = *check.DB
	} else if db, ok := connection.Properties["db"]; ok {
		if dbInt, err := strconv.Atoi(db); nil == err {
			redisOpts.DB = dbInt
		}
	}

	if check.TLSConfig.Enabled() {
		tlsConf, err := buildRedisTLSConfig(ctx, check.TLSConfig)
		if err != nil {
			return results.Failf("invalid tls config: %v", err)
		}
		redisOpts.TLSConfig = tlsConf

		// Apply handshakeTimeout as DialTimeout if explicitly configured.
		// go-redis passes DialTimeout to net.Dialer.Timeout, which covers
		// the full dial including the TLS handshake.
		if ht, err := check.TLSConfig.HandshakeTimeout.GetDurationOr(0); err == nil && ht > 0 {
			redisOpts.DialTimeout = ht
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

// buildRedisTLSConfig builds a *tls.Config for a redis connection from the
// user-supplied SwitchableTLSConfig.
func buildRedisTLSConfig(ctx *context.Context, tlsConf *v1.SwitchableTLSConfig) (*tls.Config, error) {
	cfg, err := tlsConf.TLSConfig.ToTLSConfig(ctx, ctx.GetNamespace())
	if err != nil {
		return nil, err
	}
	cfg.MinVersion = tls.VersionTLS12
	return cfg, nil
}
