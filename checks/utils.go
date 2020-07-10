package checks

import (
	"fmt"
	"sync"

	"github.com/flanksource/canary-checker/pkg"
)

func unexpectedErrorf(check pkg.GenericCheck, err error, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    false,
		Invalid: false,
		Message: fmt.Sprintf("unexpected error %s: %v", fmt.Sprintf(msg, args...), err),
	}
}

func invalidErrorf(check pkg.GenericCheck, err error, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    false,
		Invalid: true,
		Message: fmt.Sprintf("%s: %v", fmt.Sprintf(msg, args...), err),
	}
}

func Failf(check pkg.GenericCheck, msg string, args ...interface{}) *pkg.CheckResult {
	return &pkg.CheckResult{
		Check:   check,
		Pass:    false,
		Invalid: false,
		Message: fmt.Sprintf(msg, args...),
	}
}

func Passf(check pkg.GenericCheck, msg string, args ...interface{}) *pkg.CheckResult {
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
