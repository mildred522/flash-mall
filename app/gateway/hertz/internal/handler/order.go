package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CreateOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req CreateOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order create request"))
			return
		}
		req.RequestID = strings.TrimSpace(req.RequestID)
		req.UserID = identity.UserID
		if len(req.RequestID) > 64 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "request_id must be <= 64 characters"))
			return
		}
		if req.ProductID <= 0 || req.Amount <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id and positive amount are required"))
			return
		}
		if req.ExpectedPriceFen < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "expected_price_fen must be non-negative"))
			return
		}
		if strings.TrimSpace(svcCtx.Config.DtmServer) == "" || strings.TrimSpace(svcCtx.Config.OrderRpcTarget) == "" {
			fail(ctx, c, consts.StatusBadGateway, apperror.New(apperror.CodeInternal, "order saga is not configured"))
			return
		}

		if req.RequestID != "" {
			if resp, found, err := loadCreateOrderRespByRequestID(ctx, svcCtx, req.RequestID, req.UserID); err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order idempotency lookup failed", err))
				return
			} else if found {
				ok(ctx, c, resp)
				return
			}
		}

		orderID := orderIDForRequest(req.RequestID)
		if err := submitCreateOrderSaga(svcCtx, req, orderID); err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}

		resp, err := loadCreateOrderRespByOrderID(ctx, svcCtx, orderID, req.UserID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func PayOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req PayOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order pay request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}

		payment, err := loadUserPaymentOrder(ctx, svcCtx, req.OrderID, identity.UserID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		if !orderstatus.CanPay(payment.OrderStatus) && payment.OrderStatus != orderstatus.Paid {
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeOrderStatusInvalid, "order is not payable"))
			return
		}
		resp, err := buildPaymentIntentResp(c, svcCtx, payment, time.Now())
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func OrderListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order query failed", err))
			return
		}
		rows, err := db.QueryContext(ctx, `
SELECT o.id,
       o.product_id,
       o.amount,
       o.status,
       DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s'),
       s.product_name,
       s.payable_amount_fen
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
WHERE o.user_id = ?
ORDER BY o.create_time DESC
LIMIT 50`, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order query failed", err))
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]OrderListItem, 0)
		for rows.Next() {
			var item OrderListItem
			if err := rows.Scan(&item.OrderID, &item.ProductID, &item.Amount, &item.Status, &item.CreateTime, &item.ProductName, &item.PayableAmountFen); err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order scan failed", err))
				return
			}
			item.StatusText = orderstatus.Text(item.Status)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order query failed", err))
			return
		}
		ok(ctx, c, OrderListResp{Items: items})
	}
}

func OrderDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		orderID := strings.TrimSpace(c.Query("order_id"))
		if orderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}

		resp, err := loadUserOrderDetail(ctx, svcCtx, orderID, identity.UserID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func CancelOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req CancelOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order cancel request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "user cancel"
		}

		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.CancelUser(ctx, ports.CancelUserOrderCommand{
				OrderID: req.OrderID, Reason: req.Reason, UserID: identity.UserID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, CancelOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Closed)})
	}
}

func RefundOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req RefundOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order refund request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "user refund"
		}

		if err := requestUserRefund(ctx, svcCtx, req, identity.UserID); err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, RefundOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.RefundRequested)})
	}
}

func ConfirmReceiptHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req ConfirmReceiptReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid confirm receipt request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}

		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.ConfirmReceipt(ctx, ports.ConfirmReceiptCommand{
				OrderID: req.OrderID, UserID: identity.UserID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, ConfirmReceiptResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Completed)})
	}
}

func submitCreateOrderSaga(svcCtx *svc.ServiceContext, req CreateOrderReq, orderID string) error {
	gid := dtmgrpc.MustGenGid(svcCtx.Config.DtmServer)
	saga := dtmgrpc.NewSagaGrpc(svcCtx.Config.DtmServer, gid)
	saga.WaitResult = true
	if svcCtx.Config.DtmTimeoutToFailSeconds > 0 {
		saga.TimeoutToFail = svcCtx.Config.DtmTimeoutToFailSeconds
	}
	if svcCtx.Config.DtmRequestTimeoutSeconds > 0 {
		saga.WithGlobalTransRequestTimeout(svcCtx.Config.DtmRequestTimeoutSeconds)
	}
	if svcCtx.Config.DtmWaitResult {
		saga.WaitResult = true
	}

	orderRoute := strings.TrimRight(svcCtx.Config.OrderRpcTarget, "/") + "/order.Order"
	saga.Add(orderRoute+"/PreDeduct", orderRoute+"/PreDeductRollback", &orderpb.PreDeductReq{
		ProductId: req.ProductID,
		Amount:    req.Amount,
		OrderId:   orderID,
	})
	saga.Add(orderRoute+"/CreateOrder", orderRoute+"/CreateOrderRollback", &orderpb.CreateOrderReq{
		OrderId:          orderID,
		RequestId:        requestIDForOrder(req.RequestID, orderID),
		UserId:           req.UserID,
		ProductId:        req.ProductID,
		Amount:           req.Amount,
		ExpectedPriceFen: req.ExpectedPriceFen,
	})
	if err := saga.Submit(); err != nil {
		return status.Error(codes.Unavailable, "order system busy")
	}
	return nil
}

func requestUserRefund(ctx context.Context, svcCtx *svc.ServiceContext, req RefundOrderReq, userID int64) error {
	requestID := tracectx.RequestIDFrom(ctx)
	if requestID == "" {
		requestID = req.OrderID + ":refund-request"
	}
	_, err := svcCtx.OrderRpc.RequestRefund(ctx, &orderpb.RequestRefundReq{
		OrderId: req.OrderID, RequesterId: userID, RequesterRole: "user",
		Reason: req.Reason, RequestId: requestID,
	})
	return err
}

func loadUserOrderDetail(ctx context.Context, svcCtx *svc.ServiceContext, orderID string, userID int64) (OrderDetailResp, error) {
	db, err := orderDB(svcCtx)
	if err != nil {
		return OrderDetailResp{}, err
	}
	var resp OrderDetailResp
	err = db.QueryRowContext(ctx, `
SELECT o.id,
       o.user_id,
       COALESCE(o.merchant_id, 0),
       COALESCE(m.name, ''),
       o.product_id,
       o.amount,
       o.status,
       DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s'),
       s.product_name,
       s.origin_unit_price_fen,
       s.sale_unit_price_fen,
       s.payable_amount_fen,
       s.discount_amount_fen,
       s.promotion_type,
       s.promotion_tag,
       p.id,
       p.status
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
LEFT JOIN merchant m ON m.id = o.merchant_id
WHERE o.id = ? AND o.user_id = ?
LIMIT 1`, orderID, userID).Scan(
		&resp.OrderID,
		&resp.UserID,
		&resp.MerchantID,
		&resp.MerchantName,
		&resp.ProductID,
		&resp.Amount,
		&resp.Status,
		&resp.CreateTime,
		&resp.ProductName,
		&resp.OriginUnitPriceFen,
		&resp.SaleUnitPriceFen,
		&resp.PayableAmountFen,
		&resp.DiscountAmountFen,
		&resp.PromotionType,
		&resp.PromotionTag,
		&resp.PaymentOrderID,
		&resp.PaymentStatus,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrderDetailResp{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return OrderDetailResp{}, err
	}
	resp.StatusText = orderstatus.Text(resp.Status)
	resp.PaymentStatusText = paymentstatus.Text(resp.PaymentStatus)
	return resp, nil
}

type userPaymentOrder struct {
	OrderID          string
	UserID           int64
	OrderStatus      int64
	PaymentOrderID   string
	PaymentStatus    int64
	OutTradeNo       string
	PayableAmountFen int64
}

func loadUserPaymentOrder(ctx context.Context, svcCtx *svc.ServiceContext, orderID string, userID int64) (userPaymentOrder, error) {
	db, err := orderDB(svcCtx)
	if err != nil {
		return userPaymentOrder{}, err
	}
	var payment userPaymentOrder
	err = db.QueryRowContext(ctx, `
SELECT o.id,
       o.user_id,
       o.status,
       p.id,
       p.status,
       p.out_trade_no,
       p.payable_amount_fen
FROM orders o
JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ? AND o.user_id = ?
LIMIT 1`, orderID, userID).Scan(
		&payment.OrderID,
		&payment.UserID,
		&payment.OrderStatus,
		&payment.PaymentOrderID,
		&payment.PaymentStatus,
		&payment.OutTradeNo,
		&payment.PayableAmountFen,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userPaymentOrder{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return userPaymentOrder{}, err
	}
	return payment, nil
}

func loadCreateOrderRespByRequestID(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID int64) (CreateOrderResp, bool, error) {
	db, err := orderDB(svcCtx)
	if err != nil {
		return CreateOrderResp{}, false, err
	}
	var orderID string
	err = db.QueryRowContext(ctx, "SELECT id FROM orders WHERE request_id = ? AND user_id = ? LIMIT 1", requestID, userID).Scan(&orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CreateOrderResp{}, false, nil
		}
		return CreateOrderResp{}, false, err
	}
	resp, err := loadCreateOrderRespByOrderID(ctx, svcCtx, orderID, userID)
	if err != nil {
		return CreateOrderResp{}, false, err
	}
	return resp, true, nil
}

func loadCreateOrderRespByOrderID(ctx context.Context, svcCtx *svc.ServiceContext, orderID string, userID int64) (CreateOrderResp, error) {
	db, err := orderDB(svcCtx)
	if err != nil {
		return CreateOrderResp{}, err
	}
	var resp CreateOrderResp
	var statusCode int64
	err = db.QueryRowContext(ctx, `
SELECT o.id,
       o.status,
       COALESCE(s.payable_amount_fen, 0),
       COALESCE(p.id, '')
FROM orders o
LEFT JOIN order_price_snapshot s ON s.order_id = o.id
LEFT JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ? AND o.user_id = ?
LIMIT 1`, orderID, userID).Scan(&resp.OrderID, &statusCode, &resp.PayableAmountFen, &resp.PaymentOrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CreateOrderResp{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return CreateOrderResp{}, err
	}
	resp.Status = orderstatus.Text(statusCode)
	if resp.PaymentOrderID == "" {
		resp.PaymentOrderID = "pay:" + resp.OrderID
	}
	return resp, nil
}

func orderDB(svcCtx *svc.ServiceContext) (*sql.DB, error) {
	if svcCtx.OrderSqlConn == nil {
		return nil, apperror.New(apperror.CodeInternal, "order datasource is not configured")
	}
	return svcCtx.OrderSqlConn.RawDB()
}

func orderIDForRequest(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return requestID
	}
	return fmt.Sprintf("o-%d", time.Now().UnixNano())
}

func requestIDForOrder(requestID string, orderID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return requestID
	}
	return orderID
}

func createOrderStatusCode(err error) int {
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return consts.StatusBadRequest
	case apperror.CodeUnauthorized:
		return consts.StatusUnauthorized
	case apperror.CodeForbidden:
		return consts.StatusForbidden
	case apperror.CodeOrderNotFound, apperror.CodeRefundNotFound, apperror.CodeNotFound:
		return consts.StatusNotFound
	case apperror.CodeOrderStatusInvalid, apperror.CodeRefundNotAllowed, apperror.CodeRefundStatusInvalid, apperror.CodeConflict:
		return consts.StatusConflict
	case apperror.CodeStockInsufficient:
		return consts.StatusConflict
	case apperror.CodeStockNotFound, apperror.CodeProductNotFound:
		return consts.StatusNotFound
	case apperror.CodeStockReserveFailed, apperror.CodeStockReconcileFailed:
		return consts.StatusServiceUnavailable
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.InvalidArgument:
			return consts.StatusBadRequest
		case codes.FailedPrecondition, codes.Aborted:
			return consts.StatusConflict
		case codes.NotFound:
			return consts.StatusNotFound
		case codes.Unauthenticated:
			return consts.StatusUnauthorized
		case codes.PermissionDenied:
			return consts.StatusForbidden
		case codes.Unavailable:
			return consts.StatusServiceUnavailable
		}
	}
	return consts.StatusBadGateway
}
