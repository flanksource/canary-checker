package echo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echopprof "github.com/sevennt/echo-pprof"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

var Debug bool

var AllowedCORS string

func New(ctx context.Context) *echo.Echo {
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{AllowedCORS},
	}))

	if Debug {
		logger.Infof("Starting pprof at /debug")
		echopprof.Wrap(e)
	}

	e.Use(otelecho.Middleware("canary-checker", otelecho.WithSkipper(tracingURLSkipper)))

	echoLogConfig := middleware.DefaultLoggerConfig
	echoLogConfig.Skipper = tracingURLSkipper

	e.Use(middleware.LoggerWithConfig(echoLogConfig))

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(ctx.Wrap(c.Request().Context())))
			return next(c)
		}
	})

	e.GET("/api/summary", api.HealthSummary) // Deprecated: Use Post request for filtering
	e.POST("/api/summary", api.HealthSummary)
	e.GET("/about", api.About)
	e.GET("/api/graph", api.CheckDetails)
	e.POST("/api/push", api.PushHandler)
	e.GET("/api/details", api.DetailsHandler)
	e.GET("/api/topology", api.Topology)

	e.POST("/webhook/:id", api.WebhookHandler)

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	e.POST("/run/check/:id", api.RunCanaryHandler)
	e.POST("/run/topology/:id", api.RunTopologyHandler)
	return e
}

func Forward(e *echo.Echo, prefix string, target string, respModifierFunc func(*http.Response) error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		e.Logger.Fatal(err)
	}
	e.Group(prefix).Use(middleware.ProxyWithConfig(middleware.ProxyConfig{
		Rewrite: map[string]string{
			fmt.Sprintf("^%s/*", prefix): "/$1",
		},
		Balancer: middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
			{
				URL: targetURL,
			},
		}),
		ModifyResponse: respModifierFunc,
	}))
}

// tracingURLSkipper ignores metrics route on some middleware
func tracingURLSkipper(c echo.Context) bool {
	pathsToSkip := []string{"/health", "/metrics"}
	for _, p := range pathsToSkip {
		if strings.HasPrefix(c.Path(), p) {
			return true
		}
	}
	return false
}
