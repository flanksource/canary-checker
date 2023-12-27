package api_test

import (
	gocontext "context"
	"fmt"
	"net/http"
	"testing"

	apiContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/echo"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/tests/setup"
	echov4 "github.com/labstack/echo/v4"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testEchoServer       *echov4.Echo
	testEchoServerPort   = 9232
	ctx                  context.Context
	httpCheckCallCounter int
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "API Tests")
}

var _ = ginkgo.BeforeSuite(func() {

	ctx = setup.BeforeSuiteFn().WithDBLogLevel("trace").WithTrace()
	apiContext.DefaultContext = ctx
	testEchoServer = echo.New(ctx)
	cache.PostgresCache = cache.NewPostgresCache(ctx)

	// A dummy endpoint used by the HTTP check
	testEchoServer.GET("/http-check", func(c echov4.Context) error {
		httpCheckCallCounter++
		resp := map[string][]map[string]string{
			"alerts": {
				{
					"name":        "http-check",
					"icon":        "http",
					"message":     "A dummy http check",
					"description": "A dummy http check",
				},
			},
		}

		if httpCheckCallCounter > 1 {
			resp["alerts"][0]["deleted_at"] = "2023-10-30T09:00:00Z"
		}

		return c.JSON(http.StatusOK, resp)
	})

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
	if err := testEchoServer.Shutdown(gocontext.Background()); err != nil {
		ginkgo.Fail(err.Error())
	}
	setup.AfterSuiteFn()
})
