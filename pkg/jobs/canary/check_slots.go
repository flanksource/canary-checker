package canary

import (
	"math"
	"sync"
	"time"

	"github.com/flanksource/duty/context"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
)

// propertyCheckConcurrency limits how many canaries can run at once.
// 0 (the default) means unlimited.
// The value is read once, the first time a check runs; changes need a restart.
const propertyCheckConcurrency = "check.concurrency"

var (
	// checkDrainSemaphore holds one slot for every running check.
	// It is sized so that it never limits concurrency; it only lets shutdown
	// wait for running checks by acquiring every slot.
	checkDrainSemaphore = semaphore.NewWeighted(math.MaxInt64)

	checkLimiterOnce sync.Once
	// checkLimiter is nil when concurrency is unlimited.
	checkLimiter *semaphore.Weighted
	checkLimit   int64
)

var (
	checksRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "canary_checks_running",
		Help: "The number of canaries currently running checks",
	})

	checksWaiting = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "canary_checks_waiting",
		Help: "The number of canaries waiting for a free slot (see the check.concurrency property)",
	})

	checkWaitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "canary_check_wait_seconds",
		Help:    "Time a canary waited for a free slot before running its checks",
		Buckets: []float64{0.01, 0.1, 0.5, 1, 5, 15, 30, 60, 120, 300, 600},
	})
)

func init() {
	prometheus.MustRegister(checksRunning, checksWaiting, checkWaitDuration)
}

func getCheckLimiter(ctx context.Context) (*semaphore.Weighted, int64) {
	checkLimiterOnce.Do(func() {
		limit := ctx.Properties().Int(propertyCheckConcurrency, 0)
		if limit < 0 {
			ctx.Warnf("invalid %s=%d, running checks without a concurrency limit", propertyCheckConcurrency, limit)
			return
		}
		if limit > 0 {
			checkLimit = int64(limit)
			checkLimiter = semaphore.NewWeighted(checkLimit)
		}
	})

	return checkLimiter, checkLimit
}

// acquireCheckSlot blocks until the canary is allowed to run its checks.
// The returned func must be called once the checks (and their DB writes) are done.
func acquireCheckSlot(ctx context.Context) (func(), error) {
	limiter, limit := getCheckLimiter(ctx)

	start := time.Now()
	checksWaiting.Inc()
	defer checksWaiting.Dec()

	if limiter != nil && !limiter.TryAcquire(1) {
		ctx.Logger.V(3).Infof("waiting for a free check slot (%s=%d)", propertyCheckConcurrency, limit)
		if err := limiter.Acquire(ctx, 1); err != nil {
			return nil, err
		}
	}

	// The concurrency limit is acquired first so that a shutdown only waits
	// for checks that are running, not for ones still queued behind the limit.
	if err := checkDrainSemaphore.Acquire(ctx, 1); err != nil {
		if limiter != nil {
			limiter.Release(1)
		}
		return nil, err
	}

	wait := time.Since(start)
	checkWaitDuration.Observe(wait.Seconds())
	if wait > time.Second {
		ctx.Logger.V(3).Infof("waited %s for a free check slot", wait)
	}

	checksRunning.Inc()
	return func() {
		checksRunning.Dec()
		checkDrainSemaphore.Release(1)
		if limiter != nil {
			limiter.Release(1)
		}
	}, nil
}

// AcquireAllCheckLocks blocks until every running check has finished.
// Checks that try to start afterwards stay blocked, so this must only be called on shutdown.
func AcquireAllCheckLocks(ctx context.Context) {
	ctx.Logger.V(6).Infof("acquiring all check locks")
	if err := checkDrainSemaphore.Acquire(ctx, math.MaxInt64); err != nil {
		ctx.Logger.Errorf("failed to acquire check semaphores: %v", err)
	}
	ctx.Logger.V(6).Infof("acquired all check locks")
}
