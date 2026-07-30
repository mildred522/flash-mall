package middleware

import (
	"context"
	"fmt"
	"strings"

	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func Trace() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := string(c.GetHeader(tracectx.HeaderRequestID))
		if requestID == "" {
			requestID = tracectx.NewRequestID()
		}
		traceID := string(c.GetHeader(tracectx.HeaderTraceID))
		if traceID == "" {
			traceID = requestID
		}
		correlation := tracectx.Trace{
			RequestID:  requestID,
			TraceID:    traceID,
			UserID:     string(c.GetHeader(tracectx.HeaderUserID)),
			MerchantID: string(c.GetHeader(tracectx.HeaderMerchantID)),
		}
		c.Header(tracectx.HeaderRequestID, requestID)
		c.Header(tracectx.HeaderTraceID, traceID)
		correlatedCtx := tracectx.WithTrace(ctx, correlation)
		if !strings.HasPrefix(string(c.Path()), "/api/") {
			c.Next(correlatedCtx)
			return
		}

		ctx, span := otel.Tracer("flash-mall/hertz").Start(
			ctx,
			"HTTP request",
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		c.Next(tracectx.WithTrace(ctx, correlation))

		method := string(c.Method())
		route := metricRoute(c.FullPath(), string(c.Path()))
		statusCode := c.Response.StatusCode()
		span.SetName(fmt.Sprintf("%s %s", method, route))
		span.SetAttributes(
			attribute.String("http.request.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", statusCode),
			attribute.String("flashmall.request_id", requestID),
			attribute.String("flashmall.business_trace_id", traceID),
		)
		if statusCode >= 500 {
			span.SetStatus(otelcodes.Error, fmt.Sprintf("HTTP %d", statusCode))
		}
	}
}
