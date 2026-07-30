package handler

import (
	"net/url"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
)

const defaultSandboxPaymentTokenTTL = 15 * time.Minute

func buildPaymentIntentFromRPC(c *app.RequestContext, svcCtx *svc.ServiceContext, payment *orderpb.CreatePaymentResp, now time.Time) (PayOrderResp, error) {
	statusText := paymentIntentStatus(payment.GetPaymentStatus())
	resp := PayOrderResp{
		OrderID: payment.GetOrderId(), PaymentOrderID: payment.GetPaymentOrderId(),
		OutTradeNo: payment.GetOutTradeNo(), PayableAmountFen: payment.GetPayableAmountFen(),
		Status: statusText, Provider: payment.GetProvider(),
		QRURL: payment.GetQrUrl(), ExpiresAt: payment.GetExpiresAt(),
	}
	if statusText == "paid" || strings.TrimSpace(resp.QRURL) != "" {
		return resp, nil
	}
	local := userPaymentOrder{
		OrderID: payment.GetOrderId(), PaymentOrderID: payment.GetPaymentOrderId(),
		OutTradeNo: payment.GetOutTradeNo(), PayableAmountFen: payment.GetPayableAmountFen(),
		ExpiresAt: payment.GetExpiresAt(),
	}
	return buildPaymentIntentResp(c, svcCtx, local, now)
}

func paymentIntentStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "success", "paid":
		return "paid"
	case "closed":
		return "closed"
	case "failed":
		return "failed"
	default:
		return "pending"
	}
}

func buildPaymentIntentResp(c *app.RequestContext, svcCtx *svc.ServiceContext, payment userPaymentOrder, now time.Time) (PayOrderResp, error) {
	statusText := sandboxPaymentStatus(payment.PaymentStatus, false)
	resp := PayOrderResp{
		OrderID: payment.OrderID, PaymentOrderID: payment.PaymentOrderID, OutTradeNo: payment.OutTradeNo,
		PayableAmountFen: payment.PayableAmountFen, Status: statusText, Provider: "local_sandbox",
	}
	if statusText == "paid" {
		return resp, nil
	}
	ttl := time.Duration(svcCtx.Config.SandboxPaymentTokenTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultSandboxPaymentTokenTTL
	}
	claims := paymentTokenClaims{
		OrderID: payment.OrderID, PaymentOrderID: payment.PaymentOrderID, OutTradeNo: payment.OutTradeNo,
		PayableAmountFen: payment.PayableAmountFen, ExpiresAt: payment.ExpiresAt,
	}
	if claims.ExpiresAt <= now.Unix() {
		claims.ExpiresAt = now.Add(ttl).Unix()
	}
	token, err := signPaymentToken(svcCtx.Config.PaymentCallbackSecret, claims)
	if err != nil {
		return PayOrderResp{}, apperror.Wrap(apperror.CodeInternal, "create payment token failed", err)
	}
	resp.ExpiresAt = claims.ExpiresAt
	resp.QRURL = paymentPublicBaseURL(c) + "/pay?token=" + url.QueryEscape(token)
	return resp, nil
}

func paymentPublicBaseURL(c *app.RequestContext) string {
	scheme := strings.TrimSpace(strings.Split(string(c.GetHeader("X-Forwarded-Proto")), ",")[0])
	if scheme != "https" {
		scheme = "http"
	}
	return scheme + "://" + strings.TrimSpace(string(c.Host()))
}
