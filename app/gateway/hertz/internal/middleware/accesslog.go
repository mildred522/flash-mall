package middleware

import (
	"context"
	"log"
	"time"

	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
)

func AccessLog() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		c.Next(ctx)
		log.Printf(
			"hertz access method=%s path=%s status=%d latency_ms=%d request_id=%s",
			c.Method(),
			c.Path(),
			c.Response.StatusCode(),
			time.Since(startedAt).Milliseconds(),
			tracectx.RequestIDFrom(ctx),
		)
	}
}
