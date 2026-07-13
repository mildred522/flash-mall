package handler

import (
	"context"

	"flash-mall/app/common/apiresponse"
	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func ok(ctx context.Context, c *app.RequestContext, data any) {
	c.JSON(consts.StatusOK, apiresponse.OK(tracectx.RequestIDFrom(ctx), data))
}

func fail(ctx context.Context, c *app.RequestContext, statusCode int, err error) {
	c.JSON(statusCode, apiresponse.Fail(tracectx.RequestIDFrom(ctx), err))
}
