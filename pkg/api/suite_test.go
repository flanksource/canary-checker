package api_test

import (
	gocontext "context"
	"fmt"
	"net/http"
	"testing"

	embeddedPG "github.com/fergusstrange/embedded-postgres"
	apiContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/testutils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var (
	testEchoServer     *echo.Echo
	testEchoServerPort = 9232
	dbPort             = 9999
	ctx                context.Context

	testDB   *gorm.DB
	testPool *pgxpool.Pool

	postgresServer *embeddedPG.EmbeddedPostgres
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "API Tests")
}

var _ = ginkgo.BeforeSuite(func() {
	var err error

	config, dbString := testutils.GetEmbeddedPGConfig("test_canary_job", dbPort)
	postgresServer = embeddedPG.NewDatabase(config)
	if err = postgresServer.Start(); err != nil {
		ginkgo.Fail(err.Error())
	}
	logger.Infof("Started postgres on port: %d", dbPort)

	if testDB, testPool, err = duty.SetupDB(dbString, nil); err != nil {
		ginkgo.Fail(err.Error())
	}
	cache.PostgresCache = cache.NewPostgresCache(testPool)

	// Set this because some functions directly use db.Gorm
	db.Gorm = testDB
	db.Pool = testPool

	ctx = context.NewContext(gocontext.Background()).WithDB(testDB, testPool)
	apiContext.DefaultContext = ctx

	testEchoServer = echo.New()
	testEchoServer.POST("/webhook/:id", api.WebhookHandler)
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

	logger.Infof("Stopping postgres")
	if err := postgresServer.Stop(); err != nil {
		ginkgo.Fail(err.Error())
	}
})
