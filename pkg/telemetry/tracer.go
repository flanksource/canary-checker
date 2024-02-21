package telemetry

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/credentials"

	"github.com/flanksource/commons/logger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(serviceName, collectorURL string, insecure bool) func() {
	var client otlptrace.Client
	if strings.HasPrefix(collectorURL, "http") {
		client = otlptracehttp.NewClient(
			otlptracehttp.WithInsecure(),
			otlptracehttp.WithEndpoint(strings.ReplaceAll(collectorURL, "https://", "")),
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
			otlptracehttp.WithTLSClientConfig(&tls.Config{}))
	} else {
		var secureOption otlptracegrpc.Option

		if !insecure {
			secureOption = otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
		} else {
			secureOption = otlptracegrpc.WithInsecure()
		}

		client = otlptracegrpc.NewClient(
			secureOption,
			otlptracegrpc.WithEndpoint(collectorURL),
		)
	}

	exporter, err := otlptrace.New(
		context.Background(),
		client,
	)

	if err != nil {
		logger.Errorf("Failed to create opentelemetry exporter: %v", err)
		return func() {}
	}
	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		logger.Errorf("Could not set opentelemetry resources: %v", err)
		return func() {}
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)

	// Register the TraceContext propagator globally.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		err := exporter.Shutdown(ctx)
		if err != nil {
			logger.Errorf(err.Error())
		}
		defer cancel()
	}
}
