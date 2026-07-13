package middleware

import (
	"context"
	"fmt"

	"flash-mall/app/common/apiresponse"
	"flash-mall/app/common/apperror"
	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func Recover() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if recovered := recover(); recovered != nil {
				c.AbortWithStatusJSON(consts.StatusInternalServerError, apiresponse.Fail(
					tracectx.RequestIDFrom(ctx),
					apperror.New(apperror.CodeInternal, fmt.Sprintf("panic: %v", recovered)),
				))
			}
		}()
		c.Next(ctx)
	}
}
