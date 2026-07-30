package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type createPaymentRecord struct {
	OrderStatus   int64  `db:"order_status"`
	PaymentID     string `db:"payment_id"`
	OutTradeNo    string `db:"out_trade_no"`
	AmountFen     int64  `db:"amount"`
	PaymentStatus int64  `db:"payment_status"`
	Provider      string `db:"provider"`
	QRURL         string `db:"provider_qr_url"`
	ExpiresAtUnix int64  `db:"expires_at_unix"`
}

func NewCreatePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePaymentLogic {
	return &CreatePaymentLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreatePaymentLogic) CreatePayment(in *orderpb.CreatePaymentReq) (*orderpb.CreatePaymentResp, error) {
	if in == nil || strings.TrimSpace(in.GetOrderId()) == "" || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id are required")
	}
	spanCtx, span := startOrderSpan(
		l.ctx,
		"create_payment",
		attribute.String("order.id", in.GetOrderId()),
		attribute.Int64("user.id", in.GetUserId()),
	)
	defer span.End()
	l.ctx = spanCtx
	var record createPaymentRecord
	err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &record, `SELECT o.status AS order_status, p.id AS payment_id,
p.out_trade_no, p.payable_amount_fen AS amount, p.status AS payment_status,
COALESCE(p.provider, '') AS provider, COALESCE(p.provider_qr_url, '') AS provider_qr_url,
COALESCE(UNIX_TIMESTAMP(p.expires_at), 0) AS expires_at_unix
FROM orders o JOIN payment_order p ON p.order_id=o.id
WHERE o.id=? AND o.user_id=? LIMIT 1`, in.GetOrderId(), in.GetUserId())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "payment order not found")
		}
		return nil, status.Error(codes.Internal, "query payment order failed")
	}
	if record.OrderStatus != orderstatus.PendingPayment && record.OrderStatus != orderstatus.Paid {
		return nil, status.Error(codes.FailedPrecondition, "order is not payable")
	}
	resp := &orderpb.CreatePaymentResp{
		OrderId: in.GetOrderId(), PaymentOrderId: record.PaymentID, OutTradeNo: record.OutTradeNo,
		PayableAmountFen: record.AmountFen, PaymentStatus: strings.ToLower(paymentstatus.Text(record.PaymentStatus)),
	}
	if record.PaymentStatus == paymentstatus.Success {
		resp.Provider = record.Provider
		return resp, nil
	}
	expireMinutes := l.svcCtx.Config.PaymentExpireMinutes
	if expireMinutes <= 0 {
		expireMinutes = 15
	}
	if l.svcCtx.PaymentProvider == nil {
		expiresAt := time.Now().Add(time.Duration(expireMinutes) * time.Minute)
		if record.Provider == paymentprovider.NameLocalSandbox && record.ExpiresAtUnix > time.Now().Unix() {
			expiresAt = time.Unix(record.ExpiresAtUnix, 0)
		} else {
			update, updateErr := l.svcCtx.SqlConn.ExecCtx(l.ctx, `UPDATE payment_order SET provider=?,
provider_status='WAIT_BUYER_PAY', expires_at=DATE_ADD(NOW(), INTERVAL ? MINUTE), update_time=NOW()
WHERE id=? AND status=?`,
				paymentprovider.NameLocalSandbox, expireMinutes, record.PaymentID, record.PaymentStatus)
			if updateErr != nil {
				return nil, status.Error(codes.Internal, "save local payment expiry failed")
			}
			if rows, rowsErr := update.RowsAffected(); rowsErr != nil || rows != 1 {
				return nil, status.Error(codes.Aborted, "payment order changed concurrently")
			}
		}
		resp.Provider = paymentprovider.NameLocalSandbox
		resp.ExpiresAt = expiresAt.Unix()
		return resp, nil
	}
	if record.Provider == l.svcCtx.PaymentProvider.Name() && record.QRURL != "" && record.ExpiresAtUnix > time.Now().Unix() {
		resp.Provider, resp.QrUrl, resp.ExpiresAt = record.Provider, record.QRURL, record.ExpiresAtUnix
		return resp, nil
	}
	return l.createProviderPayment(resp, expireMinutes, record.PaymentStatus)
}

func (l *CreatePaymentLogic) createProviderPayment(resp *orderpb.CreatePaymentResp, expireMinutes int, currentStatus int64) (*orderpb.CreatePaymentResp, error) {
	result, err := l.svcCtx.PaymentProvider.Precreate(l.ctx, paymentprovider.PrecreateRequest{
		OutTradeNo: resp.OutTradeNo, Subject: "Flash Mall order " + resp.OrderId,
		AmountFen: resp.PayableAmountFen, ExpireMinutes: expireMinutes,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "create provider payment failed")
	}
	expiresAt := time.Now().Add(time.Duration(expireMinutes) * time.Minute)
	update, err := l.svcCtx.SqlConn.ExecCtx(l.ctx, `UPDATE payment_order SET provider=?,
provider_qr_url=?, provider_status='WAIT_BUYER_PAY',
expires_at=DATE_ADD(NOW(), INTERVAL ? MINUTE), update_time=NOW()
WHERE out_trade_no=? AND id=? AND status=?`,
		l.svcCtx.PaymentProvider.Name(), result.QRCode, expireMinutes,
		resp.OutTradeNo, resp.PaymentOrderId, currentStatus)
	if err != nil {
		return nil, status.Error(codes.Internal, "save provider payment failed")
	}
	if rows, err := update.RowsAffected(); err != nil || rows != 1 {
		return nil, status.Error(codes.Aborted, "payment order changed concurrently")
	}
	resp.Provider, resp.QrUrl, resp.ExpiresAt = l.svcCtx.PaymentProvider.Name(), result.QRCode, expiresAt.Unix()
	return resp, nil
}
