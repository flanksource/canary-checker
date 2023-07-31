package canary

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	embeddedPG "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/testutils"
	"github.com/labstack/echo/v4"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testEchoServer     *echo.Echo
	testEchoServerPort = 9232
	requestCount       int

	postgresServer *embeddedPG.EmbeddedPostgres
)

func TestCanarySyncJob(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Sync Canary Job test")
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
	var err error

	port := 9881
	config, dbString := testutils.GetEmbeddedPGConfig("test_canary_job", port)
	postgresServer = embeddedPG.NewDatabase(config)
	if err = postgresServer.Start(); err != nil {
		ginkgo.Fail(err.Error())
	}
	logger.Infof("Started postgres on port: %d", port)

	if db.Gorm, db.Pool, err = duty.SetupDB(dbString, nil); err != nil {
		ginkgo.Fail(err.Error())
	}
	cache.PostgresCache = cache.NewPostgresCache(db.Pool)

	testEchoServer = echo.New()
	testEchoServer.GET("/", DelayedResponseHandler)
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
	if err := testEchoServer.Shutdown(context.Background()); err != nil {
		ginkgo.Fail(err.Error())
	}

	logger.Infof("Stopping postgres")
	if err := postgresServer.Stop(); err != nil {
		ginkgo.Fail(err.Error())
	}
})
