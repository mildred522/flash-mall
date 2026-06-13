package observability

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/trace"
)

func TraceFields(ctx context.Context) []logx.LogField {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil
	}
	return []logx.LogField{
		logx.Field("trace_id", spanCtx.TraceID().String()),
		logx.Field("span_id", spanCtx.SpanID().String()),
	}
}

func OrderFields(ctx context.Context, orderID, requestID string) []logx.LogField {
	fields := TraceFields(ctx)
	if orderID != "" {
		fields = append(fields, logx.Field("order_id", orderID))
	}
	if requestID != "" {
		fields = append(fields, logx.Field("request_id", requestID))
	}
	return fields
}
