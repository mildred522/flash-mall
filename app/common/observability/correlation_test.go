package observability

import (
	"context"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTraceFieldsWithoutSpan(t *testing.T) {
	fields := TraceFields(context.Background())
	if len(fields) != 0 {
		t.Fatalf("expected no fields, got %#v", fields)
	}
}

func TestOrderFieldsIncludesBusinessFields(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	fields := OrderFields(ctx, "order-1", "req-1")
	names := map[string]bool{}
	for _, field := range fields {
		names[field.Key] = true
	}

	for _, key := range []string{"trace_id", "span_id", "order_id", "request_id"} {
		if !names[key] {
			t.Fatalf("missing field %s in %#v", key, fields)
		}
	}
}

func TestBusinessFieldsSkipEmptyValues(t *testing.T) {
	fields := append([]logx.LogField{}, OrderFields(context.Background(), "", "")...)
	if len(fields) != 0 {
		t.Fatalf("expected no fields, got %#v", fields)
	}
}
