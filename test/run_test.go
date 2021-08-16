package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/deps"
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
)

// nolint: errcheck
func setup() {
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

func TestRunChecks(t *testing.T) {
	files, _ := ioutil.ReadDir("../fixtures")
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
	config := pkg.ParseConfig(fmt.Sprintf("../fixtures/%s", name))
	canary := v1.Canary{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "podinfo-test",
			Name:      cleanupFilename(name),
		},
		Spec: config,
	}
	t.Run(name, func(t *testing.T) {
		checkResults := cmd.RunChecks(canary)
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
	})
}

func cleanupFilename(fileName string) string {
	removeSuffix := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	return strings.Replace(removeSuffix, "_", "", -1)
}
