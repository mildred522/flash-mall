package handler

import (
	"context"
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AlipayPaymentNotificationHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		started := time.Now()
		result := "error"
		defer func() { recordPaymentCallback(result, time.Since(started)) }()
		rawBody := string(c.Request.Body())
		resp, err := svcCtx.OrderRpc.HandlePaymentNotification(ctx, &orderpb.HandlePaymentNotificationReq{
			Provider: "alipay_sandbox", RawBody: rawBody,
		})
		c.Header("Content-Type", "text/plain; charset=utf-8")
		if err != nil || !resp.GetAccepted() {
			result = "invalid_payload"
			c.String(consts.StatusOK, "failure")
			return
		}
		result = "success"
		c.String(consts.StatusOK, "success")
	}
}
