package canary

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/tests/setup"
	"github.com/labstack/echo/v4"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testEchoServer     *echo.Echo
	testEchoServerPort int
	requestCount       int

	DefaultContext dutyContext.Context
)

func TestCanaryJobs(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Canary Job")
}

// DelayedResponseHandler waits for "delay" seconds before responding.
// It's used as a test server for HTTP check.
func DelayedResponseHandler(c echo.Context) error {
	requestCount++
	logger.Debugf("DelayedResponseHandler called: %d", requestCount)
	delayStr := c.QueryParam("delay")
	delay, err := strconv.Atoi(delayStr)
	if err != nil {
		return err
	}

	time.Sleep(time.Duration(delay) * time.Second)
	return c.String(http.StatusOK, "Done")
}

var _ = ginkgo.BeforeSuite(func() {
	DefaultContext = setup.BeforeSuiteFn().WithDBLogLevel("trace").WithTrace()

	cache.PostgresCache = cache.NewPostgresCache(DefaultContext)

	testEchoServer = echo.New()
	testEchoServer.GET("/", DelayedResponseHandler)
	testEchoServerPort = utils.FreePort()
	listenAddr := fmt.Sprintf(":%d", testEchoServerPort)

	go func() {
		defer ginkgo.GinkgoRecover() // Required by ginkgo, if an assertion is made in a goroutine.
		if err := testEchoServer.Start(listenAddr); err != nil {
			if err == http.ErrServerClosed {
				logger.Infof("Server closed")
			} else {
				ginkgo.Fail(fmt.Sprintf("Failed to start test server: %v", err))
			}
		}
	}()
})

var _ = ginkgo.AfterSuite(func() {
	logger.Infof("Stopping test echo server")
	if err := testEchoServer.Close(); err != nil {
		ginkgo.Fail(err.Error())
	}

	setup.AfterSuiteFn()
})
