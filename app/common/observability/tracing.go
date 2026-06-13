package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type ShutdownFunc func(context.Context) error

func SetupTracing(ctx context.Context, cfg TracingConfig) (ShutdownFunc, error) {
	if !cfg.IsEnabled() {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.NormalizedSampleRatio())),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider.Shutdown, nil
}

func newTraceExporter(ctx context.Context, cfg TracingConfig) (sdktrace.SpanExporter, error) {
	switch cfg.normalizedExporter() {
	case "stdout":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "otlphttp", "otlp-http":
		opts := []otlptracehttp.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpointURL(cfg.Endpoint))
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("unsupported trace exporter: %s", cfg.Exporter)
	}
}
