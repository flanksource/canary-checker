package db

import (
	"testing"
	// +kubebuilder:scaffold:imports

	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/tests/setup"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestCanaryDB(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Canary DB Suite")
}

var (
	DefaultContext context.Context
)

var _ = ginkgo.BeforeSuite(func() {
	DefaultContext = setup.BeforeSuiteFn()
})

var _ = ginkgo.AfterSuite(setup.AfterSuiteFn)
