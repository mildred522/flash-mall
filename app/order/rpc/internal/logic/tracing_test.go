package logic

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestStartOrderSpanCreatesNamedBusinessSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	_, span := startOrderSpan(
		context.Background(),
		"create_payment",
		attribute.String("order.id", "order-1"),
	)
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "order.create_payment" {
		t.Fatalf("unexpected spans: %+v", spans)
	}
	if got := spans[0].Attributes()[0].Value.AsString(); got != "order-1" {
		t.Fatalf("order id attribute=%q", got)
	}
}
