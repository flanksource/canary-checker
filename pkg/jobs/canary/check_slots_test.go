package canary

import (
	"math"
	"sync/atomic"
	"testing"
	"time"

	dutyContext "github.com/flanksource/duty/context"
	"golang.org/x/sync/semaphore"
)

func TestCheckSlots(t *testing.T) {
	ctx := dutyContext.New()

	// Use a concurrency limit of 1 without reading properties.
	checkLimiterOnce.Do(func() {})
	checkLimit = 1
	checkLimiter = semaphore.NewWeighted(checkLimit)
	t.Cleanup(func() {
		checkLimiter, checkLimit = nil, 0
	})

	release1, err := acquireCheckSlot(ctx)
	if err != nil {
		t.Fatalf("failed to acquire first slot: %v", err)
	}

	var secondStarted atomic.Bool
	secondAcquired := make(chan func())
	go func() {
		release, err := acquireCheckSlot(ctx)
		if err != nil {
			t.Errorf("failed to acquire second slot: %v", err)
			return
		}
		secondStarted.Store(true)
		secondAcquired <- release
	}()

	time.Sleep(100 * time.Millisecond)
	if secondStarted.Load() {
		t.Fatal("second check started while the concurrency limit was reached")
	}

	release1()
	var release2 func()
	select {
	case release2 = <-secondAcquired:
	case <-time.After(time.Second):
		t.Fatal("second check did not start after the first released its slot")
	}

	drained := make(chan struct{})
	go func() {
		AcquireAllCheckLocks(ctx)
		close(drained)
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case <-drained:
		t.Fatal("drain finished while a check was still running")
	default:
	}

	// A check that tries to start while the drain is waiting must stay blocked.
	var thirdStarted atomic.Bool
	go func() {
		if release, err := acquireCheckSlot(ctx); err == nil {
			thirdStarted.Store(true)
			release()
		}
	}()

	release2()
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("drain did not finish after the running check released its slot")
	}

	time.Sleep(100 * time.Millisecond)
	if thirdStarted.Load() {
		t.Fatal("a check started after the drain began")
	}

	// Let the blocked check through so other tests in the package aren't affected.
	checkDrainSemaphore.Release(math.MaxInt64)
	time.Sleep(100 * time.Millisecond)
}
