package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RequestRefundLogic struct {
	ctx context.Context
	*svc.ServiceContext
	logx.Logger
}

func NewRequestRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestRefundLogic {
	return &RequestRefundLogic{ctx: ctx, ServiceContext: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RequestRefundLogic) RequestRefund(in *orderpb.RequestRefundReq) (*orderpb.RequestRefundResp, error) {
	orderID := strings.TrimSpace(in.GetOrderId())
	role := strings.ToLower(strings.TrimSpace(in.GetRequesterRole()))
	if orderID == "" || in.GetRequesterId() <= 0 || (role != "user" && role != "admin") {
		return nil, status.Error(codes.InvalidArgument, "order_id, requester_id and requester_role are required")
	}
	db, err := l.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin refund transaction failed")
	}
	defer func() { _ = tx.Rollback() }()

	var userID, merchantID, productID, currentStatus int64
	var paymentOrderID, provider string
	var refundAmountFen, currentPaymentStatus int64
	err = tx.QueryRowContext(l.ctx, `
SELECT o.user_id, o.merchant_id, o.product_id, o.status,
       COALESCE(p.id,''), COALESCE(p.payable_amount_fen,0), COALESCE(p.status,0),
       COALESCE(p.provider,'local_sandbox')
FROM orders o
LEFT JOIN payment_order p ON p.order_id=o.id
WHERE o.id=? FOR UPDATE`, orderID).Scan(
		&userID, &merchantID, &productID, &currentStatus,
		&paymentOrderID, &refundAmountFen, &currentPaymentStatus, &provider,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if err != nil {
		return nil, err
	}
	if role == "user" && userID != in.GetRequesterId() {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to requester")
	}

	var existingID string
	var existingStatus int64
	err = tx.QueryRowContext(l.ctx, "SELECT id, status FROM refund_order WHERE order_id=? LIMIT 1", orderID).Scan(&existingID, &existingStatus)
	if err == nil {
		if existingStatus == refundStatusRejected {
			return nil, status.Error(codes.FailedPrecondition, "refund request was already rejected")
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return requestRefundResponse(existingID, orderID, currentStatus, existingStatus, true), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if !orderstatus.CanRequestRefund(currentStatus) || paymentOrderID == "" || currentPaymentStatus != paymentstatus.Success {
		return nil, status.Error(codes.FailedPrecondition, "order is not refundable")
	}

	refundID := refundIDForOrder(orderID)
	result, err := tx.ExecContext(l.ctx, "UPDATE orders SET status=?, refund_requested_at=NOW() WHERE id=? AND status=?", orderstatus.RefundRequested, orderID, currentStatus)
	if err != nil {
		return nil, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return nil, status.Error(codes.Aborted, "order status changed concurrently")
	}
	_, err = tx.ExecContext(l.ctx, `INSERT INTO refund_order
  (id, order_id, payment_order_id, user_id, merchant_id, product_id, refund_amount_fen, status, reason, provider)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, refundID, orderID, paymentOrderID, userID, merchantID, productID,
		refundAmountFen, refundStatusRequested, in.GetReason(), provider)
	if err != nil {
		return nil, err
	}
	if err = insertRefundStatusLog(l.ctx, tx, orderID, currentStatus, orderstatus.RefundRequested, in.GetRequesterId(), "refund requested: "+in.GetReason()); err != nil {
		return nil, err
	}
	if err = insertRefundOutbox(l.ctx, tx, "refund.requested", refundID, orderID, map[string]any{
		"requester_id": in.GetRequesterId(), "requester_role": role, "amount_fen": refundAmountFen, "request_id": in.GetRequestId(),
	}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return requestRefundResponse(refundID, orderID, orderstatus.RefundRequested, refundStatusRequested, false), nil
}
