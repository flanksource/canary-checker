package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/kommons"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/deps"
	"github.com/flanksource/commons/logger"
)

var (
	s3Fixtures = S3Fixture{
		CreateBuckets: []string{
			"tests-e2e-1",
			"tests-e2e-2",
		},
		Files: []S3FixtureFile{
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/pg/backups/date1/backup.zip",
				Size:        50,
				Age:         30 * 24 * time.Hour, // 30 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/pg/backups/date2/backup.zip",
				Size:        50,
				Age:         7 * 24 * time.Hour, // 7 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/mysql/backups/date1/mysql.zip",
				Size:        30,
				Age:         7*24*time.Hour - 10*time.Minute, // 30 days
				ContentType: "application/zip",
			},
		},
	}
	testFolder = ""
)

// nolint: errcheck
func setup() {
	testFolder = os.Getenv("TEST_FOLDER")
	if testFolder == "" {
		testFolder = "fixtures"
	}
	logger.Infof("Testing %s", testFolder)
	docker := deps.Binary("docker", "", "")
	docker("pull docker.io/library/busybox:1.30")
	docker("tag docker.io/library/busybox:1.30 ttl.sh/flanksource-busybox:1.30")
	docker("tag docker.io/library/busybox:1.30 docker.io/flanksource/busybox:1.30")
	os.Setenv("DOCKER_API_VERSION", "1.39")
	prepareS3E2E(s3Fixtures)
}

func teardown() {
	cleanupS3E2E(s3Fixtures)
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

var kommonsClient *kommons.Client

func init() {
	var err error
	kommonsClient, err = pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
	}
}

func TestRunChecks(t *testing.T) {
	files, _ := ioutil.ReadDir(fmt.Sprintf("../%s", testFolder))
	t.Logf("Folder: %s", testFolder)
	wg := sync.WaitGroup{}
	for _, fixture := range files {
		wg.Add(1)
		_fixture := fixture
		go func() {
			runFixture(t, path.Base(_fixture.Name()))
			wg.Done()
		}()
	}
	wg.Wait()
}

func runFixture(t *testing.T, name string) {
	t.Run(name, func(t *testing.T) {
		canaries, err := pkg.ParseConfig(fmt.Sprintf("../%s/%s", testFolder, name))
		if err != nil {
			t.Error(err)
			return
		}

		for _, canary := range canaries {

			if canary.Namespace == "" {
				canary.Namespace = "podinfo-test"
			}
			if canary.Name == "" {
				canary.Name = cmd.CleanupFilename(name)
			}
			context := context.New(kommonsClient, canary)

			checkResults := checks.RunChecks(context)
			for _, res := range checkResults {
				if res == nil {
					t.Errorf("Result in %v returned nil:\n", name)
				} else {
					if strings.Contains(name, "fail") && res.Pass {
						t.Errorf("Expected test to fail, but it passed: %s", res)
					} else if !strings.Contains(name, "fail") && !res.Pass {
						t.Errorf("Expected test to pass but it failed %s", res)
					} else {
						t.Logf("%v: %v", name, res.String())
					}
				}
			}
		}
	})
}
