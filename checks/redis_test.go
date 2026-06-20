package checks

import (
	"context"
	"fmt"
	"testing"

	checkContext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisChecker_Ping(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate redis container: %v", err)
		}
	})

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}

	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis mapped port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	checker := &RedisChecker{}

	canary := v1.Canary{
		Spec: v1.CanarySpec{
			Redis: []v1.RedisCheck{
				{
					Description: v1.Description{Name: "test-redis"},
					Connection: v1.Connection{
						URL: endpoint,
					},
				},
			},
		},
	}

	dCtx := dutyCtx.New()
	checkCtx := checkContext.New(dCtx, canary)

	results := checker.Run(checkCtx)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	for _, result := range results {
		if result.Error != "" {
			t.Errorf("expected no error, got: %s", result.Error)
		}
		if !result.Pass {
			t.Error("expected check to pass")
		}
	}
}
