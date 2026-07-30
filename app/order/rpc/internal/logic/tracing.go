package logic

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func startOrderSpan(ctx context.Context, operation string, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := otel.Tracer("flash-mall/order-rpc").Start(ctx, "order."+operation)
	span.SetAttributes(attributes...)
	return ctx, span
}
