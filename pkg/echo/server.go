package echo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/canary"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/topology"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	prom "github.com/prometheus/client_golang/prometheus"
	echopprof "github.com/sevennt/echo-pprof"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

	"github.com/flanksource/canary-checker/pkg/api"
)

var Debug bool

var AllowedCORS string

type requestLogEntry struct {
	Time         string `json:"time"`
	ID           string `json:"id"`
	RemoteIP     string `json:"remote_ip"`
	Host         string `json:"host"`
	Method       string `json:"method"`
	URI          string `json:"uri"`
	UserAgent    string `json:"user_agent"`
	Status       int    `json:"status"`
	Error        string `json:"error"`
	Latency      int64  `json:"latency"`
	LatencyHuman string `json:"latency_human"`
	BytesIn      int64  `json:"bytes_in"`
	BytesOut     int64  `json:"bytes_out"`
}

func New(ctx context.Context) *echo.Echo {
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{AllowedCORS},
	}))

	if Debug {
		logger.Infof("Starting pprof at /debug")
		echopprof.Wrap(e)
	}

	e.Use(otelecho.Middleware("canary-checker", otelecho.WithSkipper(telemetryURLSkipper)))
	e.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
		Registerer:                prom.DefaultRegisterer,
		Skipper:                   telemetryURLSkipper,
		DoNotUseRequestPathFor404: true,
	}))

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper:          telemetryURLSkipper,
		LogLatency:       true,
		LogRemoteIP:      true,
		LogHost:          true,
		LogMethod:        true,
		LogURI:           true,
		LogRequestID:     true,
		LogUserAgent:     true,
		LogStatus:        true,
		LogError:         true,
		LogContentLength: true,
		LogResponseSize:  true,
		HandleError:      true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			errorMessage := ""
			if v.Error != nil {
				errorMessage = v.Error.Error()
			}

			bytesIn, err := strconv.ParseInt(v.ContentLength, 10, 64)
			if err != nil {
				bytesIn = 0
			}

			if err := json.NewEncoder(c.Logger().Output()).Encode(requestLogEntry{
				Time:         time.Now().Format(time.RFC3339Nano),
				ID:           v.RequestID,
				RemoteIP:     v.RemoteIP,
				Host:         v.Host,
				Method:       v.Method,
				URI:          v.URI,
				UserAgent:    v.UserAgent,
				Status:       v.Status,
				Error:        errorMessage,
				Latency:      int64(v.Latency),
				LatencyHuman: v.Latency.String(),
				BytesIn:      bytesIn,
				BytesOut:     v.ResponseSize,
			}); err != nil {
				c.Logger().Error(err)
			}

			return nil
		},
	}))

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(ctx.Wrap(c.Request().Context())))
			return next(c)
		}
	})

	e.GET("/api/summary", canary.SummaryHandler) // Deprecated: Use Post request for filtering
	e.POST("/api/summary", canary.SummaryHandler)
	e.GET("/about", api.About)
	e.GET("/api/graph", api.CheckDetails)
	e.POST("/api/push", api.PushHandler)
	e.GET("/api/details", api.DetailsHandler)
	e.GET("/api/topology", topology.QueryHandler)

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

// telemetryURLSkipper ignores metrics route on some middleware
func telemetryURLSkipper(c echo.Context) bool {
	pathsToSkip := []string{"/health", "/metrics"}
	return slices.Contains(pathsToSkip, c.Path())
}
