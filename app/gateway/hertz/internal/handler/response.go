package handler

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/transport/httpx"

	"github.com/cloudwego/hertz/pkg/app"
)

func ok(ctx context.Context, c *app.RequestContext, data any) {
	httpx.OK(ctx, c, data)
}

func fail(ctx context.Context, c *app.RequestContext, statusCode int, err error) {
	httpx.Fail(ctx, c, statusCode, err)
}
