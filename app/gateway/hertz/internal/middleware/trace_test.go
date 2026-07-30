package middleware

import (
	"context"
	"testing"

	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceCreatesServerSpanWithCorrelationAttributes(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	h := server.Default()
	h.Use(Trace())
	h.GET("/api/shop/catalog", func(ctx context.Context, c *app.RequestContext) {
		if got := tracectx.RequestIDFrom(ctx); got != "request-123" {
			t.Fatalf("request id=%q", got)
		}
		c.Status(consts.StatusOK)
	})

	response := ut.PerformRequest(
		h.Engine,
		"GET",
		"/api/shop/catalog",
		nil,
		ut.Header{Key: tracectx.HeaderRequestID, Value: "request-123"},
	).Result()
	if response.StatusCode() != consts.StatusOK {
		t.Fatalf("status=%d", response.StatusCode())
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("span count=%d want=1", len(spans))
	}
	if got := spans[0].Name(); got != "GET /api/shop/catalog" {
		t.Fatalf("span name=%q", got)
	}
	attributes := map[string]string{}
	for _, item := range spans[0].Attributes() {
		attributes[string(item.Key)] = item.Value.AsString()
	}
	if attributes["http.request.method"] != "GET" {
		t.Fatalf("method attribute=%q", attributes["http.request.method"])
	}
	if attributes["flashmall.request_id"] != "request-123" {
		t.Fatalf("request id attribute=%q", attributes["flashmall.request_id"])
	}
}

func TestTraceSkipsProbeAndMetricsTraffic(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	h := server.Default()
	h.Use(Trace())
	for _, path := range []string{"/live", "/ready", "/metrics", "/shop"} {
		h.GET(path, func(_ context.Context, c *app.RequestContext) {
			c.Status(consts.StatusOK)
		})
		response := ut.PerformRequest(h.Engine, "GET", path, nil).Result()
		if response.StatusCode() != consts.StatusOK {
			t.Fatalf("%s status=%d", path, response.StatusCode())
		}
	}

	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("probe/static span count=%d want=0", len(spans))
	}
}
