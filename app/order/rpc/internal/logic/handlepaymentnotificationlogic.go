package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HandlePaymentNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHandlePaymentNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandlePaymentNotificationLogic {
	return &HandlePaymentNotificationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *HandlePaymentNotificationLogic) HandlePaymentNotification(in *orderpb.HandlePaymentNotificationReq) (*orderpb.HandlePaymentNotificationResp, error) {
	if in == nil || strings.TrimSpace(in.GetProvider()) != paymentprovider.NameAlipaySandbox ||
		strings.TrimSpace(in.GetRawBody()) == "" {
		return nil, status.Error(codes.InvalidArgument, "alipay provider and raw body are required")
	}
	if l.svcCtx.PaymentProvider == nil || l.svcCtx.PaymentProvider.Name() != paymentprovider.NameAlipaySandbox {
		return nil, status.Error(codes.FailedPrecondition, "alipay sandbox is not enabled")
	}
	values, err := url.ParseQuery(in.GetRawBody())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid alipay notification body")
	}
	notification, err := l.svcCtx.PaymentProvider.VerifyNotification(values)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid alipay notification signature")
	}
	if !isAlipayPaidStatus(notification.TradeStatus) {
		_, _ = l.svcCtx.SqlConn.ExecCtx(l.ctx,
			"UPDATE payment_order SET provider_status=?, update_time=NOW() WHERE out_trade_no=? AND provider=?",
			notification.TradeStatus, notification.OutTradeNo, paymentprovider.NameAlipaySandbox)
		return &orderpb.HandlePaymentNotificationResp{Accepted: true}, nil
	}
	var identity struct {
		PaymentID string `db:"payment_id"`
		OrderID   string `db:"order_id"`
	}
	err = l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &identity,
		"SELECT id AS payment_id, order_id FROM payment_order WHERE out_trade_no=? AND provider=? LIMIT 1",
		notification.OutTradeNo, paymentprovider.NameAlipaySandbox)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "payment order not found")
		}
		return nil, status.Error(codes.Internal, "query payment identity failed")
	}
	callbackBody, err := buildPaidCallback(notification)
	if err != nil {
		return nil, status.Error(codes.Internal, "build paid callback failed")
	}
	resp, err := NewMarkOrderPaidLogic(l.ctx, l.svcCtx).MarkPaid(&orderpb.MarkOrderPaidReq{
		OrderId: identity.OrderID, PaymentOrderId: identity.PaymentID,
		OutTradeNo: notification.OutTradeNo, CallbackBody: callbackBody,
	})
	if err != nil {
		return nil, err
	}
	return &orderpb.HandlePaymentNotificationResp{Accepted: true, OrderStatus: resp.GetOrderStatus()}, nil
}

func buildPaidCallback(notification paymentprovider.Notification) (string, error) {
	body, err := json.Marshal(map[string]any{
		"trade_status": "SUCCESS", "provider": paymentprovider.NameAlipaySandbox,
		"event_id": notification.EventID, "paid_amount_fen": notification.AmountFen,
		"provider_trade_no": notification.TradeNo,
	})
	return string(body), err
}

func isAlipayPaidStatus(value string) bool {
	return value == "TRADE_SUCCESS" || value == "TRADE_FINISHED"
}
