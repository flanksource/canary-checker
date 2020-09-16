package checks

import (
	"fmt"
	"sync"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg"
)

func unexpectedErrorf(check external.Check, err error, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    false,
		Invalid: false,
		Message: fmt.Sprintf("unexpected error %s: %v", fmt.Sprintf(msg, args...), err),
	}
}

func invalidErrorf(check external.Check, err error, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    false,
		Invalid: true,
		Message: fmt.Sprintf("%s: %v", fmt.Sprintf(msg, args...), err),
	}
}

func Failf(check external.Check, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    false,
		Invalid: false,
		Message: fmt.Sprintf(msg, args...),
	}
}

func Success(check external.Check, start time.Time) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Invalid:  false,
		Duration: time.Since(start).Milliseconds(),
	}
}

func Passf(check external.Check, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    true,
		Invalid: false,
		Message: fmt.Sprintf(msg, args...),
	}
}

type NameGenerator struct {
	NamespacesCount int
	PodsCount       int
	namespaceIndex  int
	podIndex        int
	mtx             sync.Mutex
}

func (n *NameGenerator) NamespaceName(prefix string) string {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	name := fmt.Sprintf("%s%d", prefix, n.namespaceIndex)
	n.namespaceIndex = (n.namespaceIndex + 1) % n.NamespacesCount
	return name
}

func (n *NameGenerator) PodName(prefix string) string {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	name := fmt.Sprintf("%s%d", prefix, n.podIndex)
	n.podIndex = (n.PodsCount + 1) % n.PodsCount
	return name
}

func age(duration time.Duration) string {
	if duration.Hours() > 24 {
		return fmt.Sprintf("%.1fd", duration.Hours()/24)
	} else if duration.Minutes() > 60 {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
	return fmt.Sprintf("%.1fm", duration.Minutes())
}

func mb(bytes int64) string {
	if bytes > 1024*1024*1024 {
		return fmt.Sprintf("%dGB", bytes/1024/1024/1024)
	} else if bytes > 1024*1024 {
		return fmt.Sprintf("%dMB", bytes/1024/1024)
	} else if bytes > 1024 {
		return fmt.Sprintf("%dKB", bytes/1024)
	}
	return fmt.Sprintf("%dB", bytes)
}
