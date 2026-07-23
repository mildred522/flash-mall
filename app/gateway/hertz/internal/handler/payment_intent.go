package handler

import (
	"net/url"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
)

const defaultSandboxPaymentTokenTTL = 15 * time.Minute

func buildPaymentIntentResp(c *app.RequestContext, svcCtx *svc.ServiceContext, payment userPaymentOrder, now time.Time) (PayOrderResp, error) {
	statusText := sandboxPaymentStatus(payment.PaymentStatus, false)
	resp := PayOrderResp{
		OrderID: payment.OrderID, PaymentOrderID: payment.PaymentOrderID, OutTradeNo: payment.OutTradeNo,
		PayableAmountFen: payment.PayableAmountFen, Status: statusText,
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
		PayableAmountFen: payment.PayableAmountFen, ExpiresAt: now.Add(ttl).Unix(),
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
