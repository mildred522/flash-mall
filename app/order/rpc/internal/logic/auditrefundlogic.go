package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuditRefundLogic struct {
	ctx context.Context
	*svc.ServiceContext
	logx.Logger
}

func NewAuditRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditRefundLogic {
	return &AuditRefundLogic{ctx: ctx, ServiceContext: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AuditRefundLogic) AuditRefund(in *orderpb.AuditRefundReq) (*orderpb.AuditRefundResp, error) {
	refundID := strings.TrimSpace(in.GetRefundId())
	if refundID == "" || in.GetOperatorId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "refund_id and operator_id are required")
	}
	db, err := l.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin refund audit transaction failed")
	}
	defer func() { _ = tx.Rollback() }()

	var orderID string
	var refundStatus, orderStatus int64
	err = tx.QueryRowContext(l.ctx, `SELECT r.order_id, r.status, o.status
FROM refund_order r JOIN orders o ON o.id=r.order_id
WHERE r.id=? FOR UPDATE`, refundID).Scan(&orderID, &refundStatus, &orderStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "refund order not found")
	}
	if err != nil {
		return nil, err
	}

	if refundStatus == refundStatusSuccess {
		if !in.GetApprove() {
			return nil, status.Error(codes.FailedPrecondition, "refund was already approved")
		}
		if err = l.releaseRefundStock(orderID); err != nil {
			return nil, status.Error(codes.Unavailable, "inventory release retry failed")
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return auditRefundResponse(refundID, orderID, orderstatus.Refunded, refundStatusSuccess, true), nil
	}
	if refundStatus == refundStatusRejected {
		if in.GetApprove() {
			return nil, status.Error(codes.FailedPrecondition, "refund was already rejected")
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return auditRefundResponse(refundID, orderID, orderStatus, refundStatusRejected, true), nil
	}
	if refundStatus != refundStatusRequested && refundStatus != refundStatusApproved && refundStatus != refundStatusFailed {
		return nil, status.Error(codes.FailedPrecondition, "refund status cannot be audited")
	}
	if orderStatus != orderstatus.RefundRequested {
		return nil, status.Error(codes.FailedPrecondition, "order is not waiting for refund audit")
	}

	if in.GetApprove() {
		return l.approveRefund(tx, in, refundID, orderID)
	}
	return l.rejectRefund(tx, in, refundID, orderID)
}

func (l *AuditRefundLogic) releaseRefundStock(orderID string) error {
	if l.InventoryClient == nil {
		return errors.New("inventory client not configured")
	}
	return l.InventoryClient.ReleaseStock(l.ctx, orderID, "refund approved")
}

func (l *AuditRefundLogic) approveRefund(tx *sql.Tx, in *orderpb.AuditRefundReq, refundID, orderID string) (*orderpb.AuditRefundResp, error) {
	if err := l.releaseRefundStock(orderID); err != nil {
		_ = tx.Rollback()
		if markErr := l.markRefundReleaseFailed(in, refundID, orderID, err); markErr != nil {
			return nil, status.Error(codes.Internal, "record inventory release failure failed")
		}
		return nil, status.Error(codes.Unavailable, "inventory release failed")
	}
	result, err := tx.ExecContext(l.ctx, `UPDATE refund_order
SET status=?, audit_remark=?, operator_id=?, audit_time=NOW(), finish_time=NOW()
WHERE id=? AND status IN (?, ?, ?)`, refundStatusSuccess, in.GetRemark(), in.GetOperatorId(), refundID,
		refundStatusRequested, refundStatusApproved, refundStatusFailed)
	if err != nil {
		return nil, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return nil, status.Error(codes.Aborted, "refund status changed concurrently")
	}
	result, err = tx.ExecContext(l.ctx, "UPDATE orders SET status=?, refunded_at=NOW() WHERE id=? AND status=?", orderstatus.Refunded, orderID, orderstatus.RefundRequested)
	if err != nil {
		return nil, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return nil, status.Error(codes.Aborted, "order status changed concurrently")
	}
	if err = insertRefundStatusLog(l.ctx, tx, orderID, orderstatus.RefundRequested, orderstatus.Refunded, in.GetOperatorId(), "refund approved: "+in.GetRemark()); err != nil {
		return nil, err
	}
	if err = insertRefundOutbox(l.ctx, tx, "refund.succeeded", refundID, orderID, map[string]any{
		"operator_id": in.GetOperatorId(), "remark": in.GetRemark(), "request_id": in.GetRequestId(),
	}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return auditRefundResponse(refundID, orderID, orderstatus.Refunded, refundStatusSuccess, false), nil
}

func (l *AuditRefundLogic) rejectRefund(tx *sql.Tx, in *orderpb.AuditRefundReq, refundID, orderID string) (*orderpb.AuditRefundResp, error) {
	restoreStatus := orderstatus.Paid
	err := tx.QueryRowContext(l.ctx, `SELECT from_status FROM order_status_log
WHERE order_id=? AND to_status=? ORDER BY id DESC LIMIT 1`, orderID, orderstatus.RefundRequested).Scan(&restoreStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if restoreStatus != orderstatus.Paid && restoreStatus != orderstatus.Shipped {
		restoreStatus = orderstatus.Paid
	}
	result, err := tx.ExecContext(l.ctx, `UPDATE refund_order
SET status=?, audit_remark=?, operator_id=?, audit_time=NOW(), finish_time=NULL
WHERE id=? AND status IN (?, ?, ?)`, refundStatusRejected, in.GetRemark(), in.GetOperatorId(), refundID,
		refundStatusRequested, refundStatusApproved, refundStatusFailed)
	if err != nil {
		return nil, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return nil, status.Error(codes.Aborted, "refund status changed concurrently")
	}
	result, err = tx.ExecContext(l.ctx, "UPDATE orders SET status=?, refund_requested_at=NULL WHERE id=? AND status=?", restoreStatus, orderID, orderstatus.RefundRequested)
	if err != nil {
		return nil, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return nil, status.Error(codes.Aborted, "order status changed concurrently")
	}
	if err = insertRefundStatusLog(l.ctx, tx, orderID, orderstatus.RefundRequested, restoreStatus, in.GetOperatorId(), "refund rejected: "+in.GetRemark()); err != nil {
		return nil, err
	}
	if err = insertRefundOutbox(l.ctx, tx, "refund.rejected", refundID, orderID, map[string]any{
		"operator_id": in.GetOperatorId(), "remark": in.GetRemark(), "request_id": in.GetRequestId(),
	}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return auditRefundResponse(refundID, orderID, restoreStatus, refundStatusRejected, false), nil
}

func (l *AuditRefundLogic) markRefundReleaseFailed(in *orderpb.AuditRefundReq, refundID, orderID string, cause error) error {
	db, err := l.SqlConn.RawDB()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	message := trimRefundError(cause)
	result, err := tx.ExecContext(l.ctx, `UPDATE refund_order
SET status=?, audit_remark=?, operator_id=?, audit_time=NOW(), finish_time=NULL
WHERE id=? AND status IN (?, ?, ?)`, refundStatusFailed, message, in.GetOperatorId(), refundID,
		refundStatusRequested, refundStatusApproved, refundStatusFailed)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return tx.Commit()
	}
	if err = insertRefundOutbox(l.ctx, tx, "refund.failed", refundID, orderID, map[string]any{
		"operator_id": in.GetOperatorId(), "error_message": message, "request_id": in.GetRequestId(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}
