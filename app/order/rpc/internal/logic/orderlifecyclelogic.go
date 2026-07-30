package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type lifecycleTransition struct {
	orderID       string
	ownerColumn   string
	ownerID       int64
	operatorID    int64
	fromStatus    int64
	toStatus      int64
	timestampKind string
	remark        string
	closePayment  bool
}

type lifecycleResult struct {
	repeated bool
	status   int64
}

func executeLifecycleTransition(ctx context.Context, svcCtx *svc.ServiceContext, command lifecycleTransition) (lifecycleResult, error) {
	if svcCtx == nil || svcCtx.SqlConn == nil {
		return lifecycleResult{}, status.Error(codes.Unavailable, "order datasource is not configured")
	}
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return lifecycleResult{}, status.Error(codes.Unavailable, "order datasource is unavailable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return lifecycleResult{}, status.Error(codes.Internal, "begin order transaction failed")
	}
	defer func() { _ = tx.Rollback() }()

	query := "SELECT status FROM orders WHERE id = ? FOR UPDATE"
	args := []any{command.orderID}
	switch command.ownerColumn {
	case "user_id":
		query = "SELECT status FROM orders WHERE id = ? AND user_id = ? FOR UPDATE"
		args = append(args, command.ownerID)
	case "merchant_id":
		query = "SELECT status FROM orders WHERE id = ? AND merchant_id = ? FOR UPDATE"
		args = append(args, command.ownerID)
	case "":
	default:
		return lifecycleResult{}, status.Error(codes.Internal, "invalid order ownership scope")
	}

	var currentStatus int64
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return lifecycleResult{}, status.Error(codes.NotFound, "order not found")
		}
		return lifecycleResult{}, status.Error(codes.Internal, "query order status failed")
	}
	if currentStatus == command.toStatus {
		if err = tx.Commit(); err != nil {
			return lifecycleResult{}, status.Error(codes.Internal, "commit repeated order command failed")
		}
		return lifecycleResult{repeated: true, status: currentStatus}, nil
	}
	if currentStatus != command.fromStatus {
		return lifecycleResult{}, status.Error(codes.FailedPrecondition, "order status does not allow this command")
	}

	update, updateArgs, err := lifecycleUpdate(command)
	if err != nil {
		return lifecycleResult{}, err
	}
	result, err := tx.ExecContext(ctx, update, updateArgs...)
	if err != nil {
		return lifecycleResult{}, status.Error(codes.Internal, "update order status failed")
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return lifecycleResult{}, status.Error(codes.Aborted, "order status changed concurrently")
	}
	if command.closePayment {
		paymentResult, paymentErr := tx.ExecContext(ctx, `UPDATE payment_order SET status=?,
provider_status=CASE WHEN provider='alipay_sandbox' THEN 'TRADE_CLOSED' ELSE 'CLOSED' END, update_time=NOW()
WHERE order_id=? AND status=?`, paymentstatus.Closed, command.orderID, paymentstatus.Init)
		if paymentErr != nil {
			return lifecycleResult{}, status.Error(codes.Internal, "close payment order failed")
		}
		if _, paymentErr = paymentResult.RowsAffected(); paymentErr != nil {
			return lifecycleResult{}, status.Error(codes.Internal, "inspect closed payment order failed")
		}
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
		command.orderID, command.fromStatus, command.toStatus, command.operatorID, command.remark,
	); err != nil {
		return lifecycleResult{}, status.Error(codes.Internal, "insert order status log failed")
	}
	if err = tx.Commit(); err != nil {
		return lifecycleResult{}, status.Error(codes.Internal, "commit order command failed")
	}
	return lifecycleResult{status: command.toStatus}, nil
}

func lifecycleUpdate(command lifecycleTransition) (string, []any, error) {
	setClause := "status = ?, update_time = NOW()"
	switch command.timestampKind {
	case "":
	case "shipped":
		setClause = "status = ?, shipped_at = NOW(), update_time = NOW()"
	case "completed":
		setClause = "status = ?, completed_at = NOW(), update_time = NOW()"
	default:
		return "", nil, status.Error(codes.Internal, "invalid order transition timestamp")
	}
	query := "UPDATE orders SET " + setClause + " WHERE id = ? AND status = ?"
	args := []any{command.toStatus, command.orderID, command.fromStatus}
	if command.ownerColumn == "merchant_id" {
		query = "UPDATE orders SET " + setClause + " WHERE id = ? AND merchant_id = ? AND status = ?"
		args = []any{command.toStatus, command.orderID, command.ownerID, command.fromStatus}
	}
	return query, args, nil
}

func commandContext(ctx context.Context, meta *orderpb.OrderCommandMeta) context.Context {
	if meta == nil {
		return ctx
	}
	requestID := strings.TrimSpace(meta.GetRequestId())
	traceID := strings.TrimSpace(meta.GetTraceId())
	if requestID == "" {
		requestID = tracectx.NewRequestID()
	}
	if traceID == "" {
		traceID = requestID
	}
	ctx = tracectx.WithTrace(ctx, tracectx.Trace{
		RequestID: requestID, TraceID: traceID,
		UserID: numericTraceID(meta.GetUserId()), MerchantID: numericTraceID(meta.GetMerchantId()),
	})
	return authctx.WithIdentity(ctx, authctx.Identity{
		UserID: meta.GetUserId(), MerchantID: meta.GetMerchantId(), Role: strings.TrimSpace(meta.GetRole()),
		IsAdmin: strings.EqualFold(strings.TrimSpace(meta.GetRole()), authctx.RoleAdmin), RequestID: requestID,
	})
}

func numericTraceID(value int64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func releaseClosedOrder(ctx context.Context, svcCtx *svc.ServiceContext, orderID, reason string) error {
	if svcCtx.InventoryClient == nil {
		return status.Error(codes.Unavailable, "inventory client is not configured")
	}
	if err := svcCtx.InventoryClient.ReleaseStock(ctx, orderID, reason); err != nil {
		logx.WithContext(ctx).Errorf("release closed order stock failed: order_id=%s err=%v", orderID, err)
		_, _ = svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_release_status=2,
inventory_release_attempts=inventory_release_attempts+1, inventory_release_error=?, update_time=NOW()
WHERE order_id=?`, trimRefundError(err), orderID)
		return status.Error(codes.Unavailable, "release order stock failed; retry the same command")
	}
	if _, err := svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_release_status=1,
inventory_release_attempts=inventory_release_attempts+1, inventory_release_error='',
inventory_released_at=NOW(), update_time=NOW() WHERE order_id=?`, orderID); err != nil {
		return status.Error(codes.Internal, "record inventory release success failed")
	}
	return nil
}

func closeProviderPayment(ctx context.Context, svcCtx *svc.ServiceContext, orderID string) error {
	if svcCtx.PaymentProvider == nil {
		return nil
	}
	var providerName, outTradeNo, providerStatus string
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return status.Error(codes.Unavailable, "payment datasource is unavailable")
	}
	err = db.QueryRowContext(ctx,
		`SELECT provider, out_trade_no, provider_status FROM payment_order WHERE order_id=? LIMIT 1`,
		orderID).Scan(&providerName, &outTradeNo, &providerStatus)
	if err != nil {
		return status.Error(codes.Unavailable, "query provider payment failed")
	}
	if providerName != paymentprovider.NameAlipaySandbox || providerStatus == "TRADE_CLOSED" {
		return nil
	}
	if err = svcCtx.PaymentProvider.Close(ctx, outTradeNo); err != nil {
		return status.Error(codes.Unavailable, "close provider payment failed")
	}
	return nil
}

type CancelUserOrderLogic struct {
	ctx context.Context
	*svc.ServiceContext
}

func NewCancelUserOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelUserOrderLogic {
	return &CancelUserOrderLogic{ctx: ctx, ServiceContext: svcCtx}
}

func (l *CancelUserOrderLogic) CancelUserOrder(in *orderpb.CancelUserOrderReq) (*orderpb.OrderCommandResp, error) {
	if in == nil || strings.TrimSpace(in.GetOrderId()) == "" || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id are required")
	}
	reason := strings.TrimSpace(in.GetReason())
	if reason == "" {
		reason = "user cancel"
	}
	ctx := commandContext(l.ctx, in.GetMeta())
	if err := closeProviderPayment(ctx, l.ServiceContext, in.GetOrderId()); err != nil {
		return nil, err
	}
	result, err := executeLifecycleTransition(ctx, l.ServiceContext, lifecycleTransition{
		orderID: in.GetOrderId(), ownerColumn: "user_id", ownerID: in.GetUserId(), operatorID: in.GetUserId(),
		fromStatus: orderstatus.PendingPayment, toStatus: orderstatus.Closed, remark: "user cancelled: " + reason,
		closePayment: true,
	})
	if err != nil {
		return nil, err
	}
	if err = releaseClosedOrder(ctx, l.ServiceContext, in.GetOrderId(), reason); err != nil {
		return nil, err
	}
	return lifecycleResponse(in.GetOrderId(), result), nil
}

type CloseAdminOrderLogic struct {
	ctx context.Context
	*svc.ServiceContext
}

func NewCloseAdminOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseAdminOrderLogic {
	return &CloseAdminOrderLogic{ctx: ctx, ServiceContext: svcCtx}
}

func (l *CloseAdminOrderLogic) CloseAdminOrder(in *orderpb.CloseAdminOrderReq) (*orderpb.OrderCommandResp, error) {
	if in == nil || strings.TrimSpace(in.GetOrderId()) == "" || in.GetOperatorId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and operator_id are required")
	}
	reason := strings.TrimSpace(in.GetReason())
	if reason == "" {
		reason = "admin close"
	}
	ctx := commandContext(l.ctx, in.GetMeta())
	if err := closeProviderPayment(ctx, l.ServiceContext, in.GetOrderId()); err != nil {
		return nil, err
	}
	result, err := executeLifecycleTransition(ctx, l.ServiceContext, lifecycleTransition{
		orderID: in.GetOrderId(), operatorID: in.GetOperatorId(), fromStatus: orderstatus.PendingPayment,
		toStatus: orderstatus.Closed, remark: "admin closed: " + reason, closePayment: true,
	})
	if err != nil {
		return nil, err
	}
	if err = releaseClosedOrder(ctx, l.ServiceContext, in.GetOrderId(), reason); err != nil {
		return nil, err
	}
	return lifecycleResponse(in.GetOrderId(), result), nil
}

type ShipAdminOrderLogic struct {
	ctx context.Context
	*svc.ServiceContext
}

func NewShipAdminOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShipAdminOrderLogic {
	return &ShipAdminOrderLogic{ctx: ctx, ServiceContext: svcCtx}
}

func (l *ShipAdminOrderLogic) ShipAdminOrder(in *orderpb.ShipAdminOrderReq) (*orderpb.OrderCommandResp, error) {
	if in == nil || strings.TrimSpace(in.GetOrderId()) == "" || in.GetOperatorId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and operator_id are required")
	}
	result, err := executeLifecycleTransition(commandContext(l.ctx, in.GetMeta()), l.ServiceContext, lifecycleTransition{
		orderID: in.GetOrderId(), operatorID: in.GetOperatorId(), fromStatus: orderstatus.Paid,
		toStatus: orderstatus.Shipped, timestampKind: "shipped", remark: "admin shipped",
	})
	if err != nil {
		return nil, err
	}
	return lifecycleResponse(in.GetOrderId(), result), nil
}

type ShipMerchantOrderLogic struct {
	ctx context.Context
	*svc.ServiceContext
}

func NewShipMerchantOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShipMerchantOrderLogic {
	return &ShipMerchantOrderLogic{ctx: ctx, ServiceContext: svcCtx}
}

func (l *ShipMerchantOrderLogic) ShipMerchantOrder(in *orderpb.ShipMerchantOrderReq) (*orderpb.OrderCommandResp, error) {
	if in == nil || strings.TrimSpace(in.GetOrderId()) == "" || in.GetMerchantId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and merchant_id are required")
	}
	result, err := executeLifecycleTransition(commandContext(l.ctx, in.GetMeta()), l.ServiceContext, lifecycleTransition{
		orderID: in.GetOrderId(), ownerColumn: "merchant_id", ownerID: in.GetMerchantId(), operatorID: in.GetMerchantId(),
		fromStatus: orderstatus.Paid, toStatus: orderstatus.Shipped, timestampKind: "shipped", remark: "merchant ship order",
	})
	if err != nil {
		return nil, err
	}
	return lifecycleResponse(in.GetOrderId(), result), nil
}

type ConfirmReceiptLogic struct {
	ctx context.Context
	*svc.ServiceContext
}

func NewConfirmReceiptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmReceiptLogic {
	return &ConfirmReceiptLogic{ctx: ctx, ServiceContext: svcCtx}
}

func (l *ConfirmReceiptLogic) ConfirmReceipt(in *orderpb.ConfirmReceiptReq) (*orderpb.OrderCommandResp, error) {
	if in == nil || strings.TrimSpace(in.GetOrderId()) == "" || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id are required")
	}
	result, err := executeLifecycleTransition(commandContext(l.ctx, in.GetMeta()), l.ServiceContext, lifecycleTransition{
		orderID: in.GetOrderId(), ownerColumn: "user_id", ownerID: in.GetUserId(), operatorID: in.GetUserId(),
		fromStatus: orderstatus.Shipped, toStatus: orderstatus.Completed, timestampKind: "completed", remark: "buyer confirmed receipt",
	})
	if err != nil {
		return nil, err
	}
	return lifecycleResponse(in.GetOrderId(), result), nil
}

func lifecycleResponse(orderID string, result lifecycleResult) *orderpb.OrderCommandResp {
	return &orderpb.OrderCommandResp{OrderId: orderID, OrderStatus: result.status, Repeated: result.repeated}
}
