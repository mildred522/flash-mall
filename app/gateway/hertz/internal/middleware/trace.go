package middleware

import (
	"context"

	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
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
		trace := tracectx.Trace{
			RequestID:  requestID,
			TraceID:    traceID,
			UserID:     string(c.GetHeader(tracectx.HeaderUserID)),
			MerchantID: string(c.GetHeader(tracectx.HeaderMerchantID)),
		}
		c.Header(tracectx.HeaderRequestID, requestID)
		c.Header(tracectx.HeaderTraceID, traceID)
		c.Next(tracectx.WithTrace(ctx, trace))
	}
}
