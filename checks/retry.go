package checks

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

const defaultCheckRetryInterval = time.Second

type checkRetriesProvider interface {
	GetCheckRetries() *v1.CheckRetries
}

type internalRetryHandler interface {
	HandlesRetriesInternally() bool
}

type retryPolicy struct {
	configured bool
	disabled   bool
	delay      time.Duration
	timeout    time.Duration
	interval   time.Duration
	maxRetries *int
}

func runCheckWithRetries(ctx *context.Context, checker Checker, check external.Check) (pkg.Results, int) {
	if handler, ok := checker.(internalRetryHandler); ok && handler.HandlesRetriesInternally() {
		return runCheckAttempt(ctx, checker, check)
	}

	policy, err := getRetryPolicy(ctx, check)
	if err != nil {
		return pkg.Invalid(check, ctx.Canary, fmt.Sprintf("invalid checkRetries: %v", err)), 0
	}

	if policy.disabled {
		return runCheckAttempt(ctx, checker, check)
	}

	if policy.delay > 0 {
		if err := sleepWithContext(ctx, policy.delay); err != nil {
			return pkg.Invalid(check, ctx.Canary, err.Error()), 0
		}
	}

	start := time.Now()
	attempts := 0
	skippedTotal := 0
	var lastResults pkg.Results

	for {
		attempts++
		if attempts > 1 {
			ctx.Debugf("retrying check %s/%s attempt %d", check.GetType(), check.GetName(), attempts)
		}

		results, skipped := runCheckAttempt(ctx, checker, check)
		skippedTotal += skipped
		lastResults = results

		if !shouldRetryResults(results) || !policy.canRetry(attempts, start) {
			addRetryData(lastResults, policy, attempts, time.Since(start))
			return lastResults, skippedTotal
		}

		if err := sleepWithContext(ctx, policy.nextInterval(start)); err != nil {
			if len(lastResults) == 0 {
				lastResults = pkg.Invalid(check, ctx.Canary, err.Error())
			}
			addRetryData(lastResults, policy, attempts, time.Since(start))
			return lastResults, skippedTotal
		}
	}
}

func runCheckAttempt(ctx *context.Context, checker Checker, check external.Check) (pkg.Results, int) {
	singleRunner, ok := checker.(SingleCheckRunner)
	if !ok {
		return pkg.Invalid(check, ctx.Canary, fmt.Sprintf("checker %s does not implement SingleCheckRunner", checker.Type())), 0
	}

	result := singleRunner.Check(ctx, check)
	transformedResults := TransformResults(ctx, result)
	skippedCount, filteredResults := filterSecretLookupRateLimitedResults(ctx, transformedResults)
	return filteredResults, skippedCount
}

func getRetryPolicy(ctx *context.Context, check external.Check) (retryPolicy, error) {
	return newRetryPolicy(mergeCheckRetries(ctx.Canary.Spec.CheckRetries, getCheckRetries(check)))
}

func getCheckRetries(check external.Check) *v1.CheckRetries {
	provider, ok := check.(checkRetriesProvider)
	if !ok {
		return nil
	}
	return provider.GetCheckRetries()
}

func mergeCheckRetries(defaults, override *v1.CheckRetries) *v1.CheckRetries {
	if defaults == nil && override == nil {
		return nil
	}
	if override != nil && override.Disabled {
		return &v1.CheckRetries{Disabled: true}
	}

	merged := &v1.CheckRetries{}
	if defaults != nil {
		merged.Delay = defaults.Delay
		merged.Timeout = defaults.Timeout
		merged.Interval = defaults.Interval
		merged.MaxRetries = defaults.MaxRetries
		merged.Disabled = defaults.Disabled
	}
	if override != nil {
		if override.Delay != nil {
			merged.Delay = override.Delay
		}
		if override.Timeout != nil {
			merged.Timeout = override.Timeout
		}
		if override.Interval != nil {
			merged.Interval = override.Interval
		}
		if override.MaxRetries != nil {
			merged.MaxRetries = override.MaxRetries
		}
	}

	return merged
}

func newRetryPolicy(config *v1.CheckRetries) (retryPolicy, error) {
	if config == nil {
		return retryPolicy{}, nil
	}

	policy := retryPolicy{
		configured: true,
		disabled:   config.Disabled,
		maxRetries: config.MaxRetries,
	}
	if policy.disabled {
		return policy, nil
	}

	var err error
	if policy.delay, err = config.GetDelay(); err != nil {
		return policy, fmt.Errorf("delay: %w", err)
	}
	if policy.timeout, err = config.GetTimeout(); err != nil {
		return policy, fmt.Errorf("timeout: %w", err)
	}
	if policy.interval, err = config.GetInterval(); err != nil {
		return policy, fmt.Errorf("interval: %w", err)
	}

	if policy.delay < 0 {
		return policy, fmt.Errorf("delay cannot be negative")
	}
	if policy.timeout < 0 {
		return policy, fmt.Errorf("timeout cannot be negative")
	}
	if policy.interval < 0 {
		return policy, fmt.Errorf("interval cannot be negative")
	}
	if policy.maxRetries != nil && *policy.maxRetries < 0 {
		return policy, fmt.Errorf("maxRetries cannot be negative")
	}

	if policy.retryEnabled() && policy.interval == 0 && config.Interval == nil {
		policy.interval = defaultCheckRetryInterval
	}

	return policy, nil
}

func (p retryPolicy) retryEnabled() bool {
	if p.disabled {
		return false
	}
	return (p.maxRetries != nil && *p.maxRetries > 0) || p.timeout > 0
}

func (p retryPolicy) canRetry(attempts int, start time.Time) bool {
	if !p.retryEnabled() {
		return false
	}
	if p.maxRetries != nil && attempts-1 >= *p.maxRetries {
		return false
	}
	if p.timeout > 0 && time.Since(start) >= p.timeout {
		return false
	}
	return true
}

func (p retryPolicy) nextInterval(start time.Time) time.Duration {
	interval := p.interval
	if p.timeout <= 0 {
		return interval
	}

	remaining := p.timeout - time.Since(start)
	if remaining <= 0 {
		return 0
	}
	if interval <= 0 || interval > remaining {
		return remaining
	}
	return interval
}

func shouldRetryResults(results pkg.Results) bool {
	if len(results) == 0 {
		return false
	}

	for _, result := range results {
		if result == nil {
			continue
		}
		if result.Invalid {
			return false
		}
		if !result.Pass {
			return true
		}
	}

	return false
}

func sleepWithContext(ctx *context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func addRetryData(results pkg.Results, policy retryPolicy, attempts int, elapsed time.Duration) {
	if !policy.configured && attempts == 1 {
		return
	}

	data := map[string]interface{}{
		"attempts":        attempts,
		"retries":         attempts - 1,
		"elapsed":         elapsed.String(),
		"elapsedMillis":   elapsed.Milliseconds(),
		"interval":        policy.interval.String(),
		"retryConfigured": policy.configured,
	}
	if policy.maxRetries != nil {
		data["maxRetries"] = *policy.maxRetries
	}
	if policy.timeout > 0 {
		data["timeout"] = policy.timeout.String()
	}
	if policy.delay > 0 {
		data["delay"] = policy.delay.String()
	}
	if policy.disabled {
		data["disabled"] = true
	}

	for _, result := range results {
		if result == nil {
			continue
		}
		result.AddData(map[string]interface{}{
			"checkRetries": data,
		})
	}
}
