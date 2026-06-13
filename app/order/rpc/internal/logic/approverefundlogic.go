package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ApproveRefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveRefundLogic {
	return &ApproveRefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApproveRefundLogic) ApproveRefund(in *order.LifecycleOrderReq) (*order.LifecycleOrderResp, error) {
	orderID := strings.TrimSpace(in.OrderId)
	if orderID == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if l.svcCtx.ProductRpc == nil {
		return nil, status.Error(codes.Internal, "product rpc not configured")
	}

	currentStatus, productID, amount, err := l.loadRefundableOrder(orderID)
	if err != nil {
		return nil, err
	}
	if currentStatus == orderStateRefunded {
		return lifecycleResp(orderID, orderStateRefunded), nil
	}
	if currentStatus != orderStateRefundRequested && currentStatus != orderStateRefundFailed {
		return nil, status.Error(codes.FailedPrecondition, "order is not waiting for refund approval")
	}

	if err := l.restoreRefundStock(orderID, productID, amount); err != nil {
		if markErr := l.markRefundFailed(orderID, currentStatus, in, err); markErr != nil {
			return nil, markErr
		}
		return lifecycleResp(orderID, orderStateRefundFailed), nil
	}

	if err := l.markRefunded(orderID, currentStatus, in); err != nil {
		return nil, err
	}
	return lifecycleResp(orderID, orderStateRefunded), nil
}

func (l *ApproveRefundLogic) loadRefundableOrder(orderID string) (statusCode, productID, amount int64, err error) {
	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return 0, 0, 0, status.Error(codes.Internal, "db connection failed")
	}

	err = db.QueryRowContext(l.ctx,
		"SELECT status, product_id, amount FROM orders WHERE id = ? LIMIT 1",
		orderID,
	).Scan(&statusCode, &productID, &amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, status.Error(codes.NotFound, "order not found")
		}
		return 0, 0, 0, err
	}
	return statusCode, productID, amount, nil
}

func (l *ApproveRefundLogic) restoreRefundStock(orderID string, productID, amount int64) error {
	if _, err := l.svcCtx.ProductRpc.RevertStock(l.ctx, &productclient.RevertStockReq{
		Id:      productID,
		Num:     amount,
		OrderId: orderID,
	}); err != nil {
		return err
	}

	if l.svcCtx.Redis == nil {
		return nil
	}
	_, err := NewPreDeductRollbackLogic(l.ctx, l.svcCtx).PreDeductRollback(&order.PreDeductReq{
		ProductId: productID,
		Amount:    amount,
		OrderId:   orderID,
	})
	return err
}

func (l *ApproveRefundLogic) markRefundFailed(orderID string, fromStatus int64, in *order.LifecycleOrderReq, cause error) error {
	return l.persistRefundTransition(orderID, fromStatus, orderStateRefundFailed, in, "order.refund.failed", "refund failed: "+trimSQLErrorMessage(cause), false)
}

func (l *ApproveRefundLogic) markRefunded(orderID string, fromStatus int64, in *order.LifecycleOrderReq) error {
	return l.persistRefundTransition(orderID, fromStatus, orderStateRefunded, in, "order.refunded", "refund approved: "+in.Reason, true)
}

func (l *ApproveRefundLogic) persistRefundTransition(orderID string, fromStatus, toStatus int64, in *order.LifecycleOrderReq, eventType, remark string, setRefundedAt bool) error {
	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return status.Error(codes.Internal, "db connection failed")
	}

	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return status.Error(codes.Internal, "begin tx failed")
	}
	defer func() { _ = tx.Rollback() }()

	updateSQL := "UPDATE orders SET status = ?, update_time = NOW() WHERE id = ? AND status IN (5, 7)"
	if setRefundedAt {
		updateSQL = "UPDATE orders SET status = ?, refunded_at = NOW(), update_time = NOW() WHERE id = ? AND status IN (5, 7)"
	}
	result, err := tx.ExecContext(l.ctx, updateSQL, toStatus, orderID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return status.Error(codes.FailedPrecondition, "order status changed concurrently")
	}

	if err := insertOrderStatusLog(l.ctx, tx, orderID, fromStatus, toStatus, in.OperatorId, remark); err != nil {
		return err
	}
	errMsg := ""
	if toStatus == orderStateRefundFailed {
		errMsg = strings.TrimPrefix(remark, "refund failed: ")
	}
	if err := insertLifecycleOutbox(l.ctx, tx, eventType, orderID, in, toStatus, errMsg); err != nil {
		return err
	}

	return tx.Commit()
}
