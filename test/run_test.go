package test

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/duty"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var testFolder string
var DefaultContext dutyContext.Context
var verbosity = 0

func TestChecks(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Canary Checks")
}

var _ = ginkgo.BeforeSuite(func() {
	var err error
	DefaultContext, _, err = duty.Start("tests", duty.ClientOnly)
	Expect(err).To(BeNil())

	logger.StandardLogger().SetLogLevel(verbosity)

})

func init() {
	defaultFolder := "fixtures/minimal"
	if os.Getenv("TEST_FOLDER") != "" {
		defaultFolder = os.Getenv("TEST_FOLDER")
	}
	flag.IntVar(&verbosity, "verbose", 0, "Add verbose logging")
	flag.StringVar(&testFolder, "test-folder", defaultFolder, "The folder containing test fixtures to run")
}

var _ = ginkgo.Describe("Canary Checks/"+testFolder, func() {
	logger.Infof("Testing %s", testFolder)
	files, _ := os.ReadDir(fmt.Sprintf("../%s", testFolder))
	wg := sync.WaitGroup{}
	for _, fixture := range files {
		name := path.Base(fixture.Name())
		if strings.HasPrefix(name, "_") || !strings.HasSuffix(name, ".yaml") || name == "kustomization.yaml" {
			continue
		}

		// Only run those fixtures that end in _pass.yaml or _fail.yaml
		if !strings.HasSuffix(name, "_pass.yaml") &&
			!strings.HasSuffix(name, "_pass.yml") &&
			!strings.HasSuffix(name, "_fail.yaml") &&
			!strings.HasSuffix(name, "_fail.yml") {
			continue
		}

		wg.Add(1)
		go func() {
			defer ginkgo.GinkgoRecover()
			runFixture(name)
			wg.Done()
		}()
	}
	wg.Wait()
})

// func TestRunChecks(t *testing.T) {
// 	logger.Infof("Testing %s", testFolder)
// 	files, _ := os.ReadDir(fmt.Sprintf("../%s", testFolder))
// 	t.Logf("Folder: %s", testFolder)
// 	wg := sync.WaitGroup{}
// 	for _, fixture := range files {
// 		name := path.Base(fixture.Name())
// 		if strings.HasPrefix(name, "_") || !strings.HasSuffix(name, ".yaml") || name == "kustomization.yaml" {
// 			continue
// 		}
// 		wg.Add(1)
// 		go func() {
// 			defer ginkgo.GinkgoRecover()
// 			runFixture(name)
// 			wg.Done()
// 		}()
// 	}
// 	wg.Wait()
// }

func runFixture(name string) {
	ginkgo.It(name, func() {
		canaries, err := pkg.ParseConfig(fmt.Sprintf("../%s/%s", testFolder, name), "")
		if err != nil {
			ginkgo.Fail(err.Error())
			return
		}

		for _, canary := range canaries {
			if canary.Namespace == "" {
				canary.Namespace = "canaries"
			}
			if canary.Name == "" {
				canary.Name = cmd.CleanupFilename(name)
			}
			context := context.New(DefaultContext, canary)

			checkResults, _, err := checks.RunChecks(context)
			if err != nil {
				ginkgo.Fail(err.Error())
				return
			}

			for _, res := range checkResults {
				if res == nil {
					ginkgo.Fail(fmt.Sprintf("Result in %v returned nil:\n", name))
				} else {
					if strings.Contains(name, "_mix") {
						logger.Infof("%v: %v", name, res.String())
					} else if strings.Contains(name, "fail") && res.Pass {
						ginkgo.Fail(fmt.Sprintf("Expected test to fail, but it passed: %s", res))
					} else if !strings.Contains(name, "fail") && !res.Pass {
						ginkgo.Fail(fmt.Sprintf("Expected test to pass but it failed %s", res))
					} else {
						logger.Infof("%v: %v", name, res.String())
					}
				}
			}
		}
	})
}
