package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminOrderListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminOrderQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadAdminOrders(ctx, db, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin order query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminOrderDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		orderID := strings.TrimSpace(c.Query("order_id"))
		if orderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		resp, err := loadAdminOrderDetail(ctx, svcCtx, orderID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminOrderStatusLogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		orderID := strings.TrimSpace(c.Query("order_id"))
		if orderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		resp, err := loadAdminOrderStatusLogs(ctx, db, orderID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin order status log query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminShipOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req MerchantShipOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid ship order request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		operatorID := gatewayOperatorID(ctx)
		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.ShipAdmin(ctx, ports.ShipAdminOrderCommand{
				OrderID: req.OrderID, OperatorID: operatorID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditOrderShipped, fmt.Sprintf("order:%s reason:%s", req.OrderID, adminAuditReasonInvalidStatus))
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditOrderShipped, fmt.Sprintf("order:%s operator:%d", req.OrderID, operatorID))
		ok(ctx, c, MerchantShipOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Shipped)})
	}
}

func AdminCloseOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req CancelOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid close order request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "admin close"
		}
		operatorID := gatewayOperatorID(ctx)
		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.CloseAdmin(ctx, ports.CloseAdminOrderCommand{
				OrderID: req.OrderID, Reason: req.Reason, OperatorID: operatorID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s reason:%s", req.OrderID, adminAuditReasonInvalidStatus))
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditOrderClosed, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderID, operatorID, req.Reason))
		ok(ctx, c, CancelOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Closed)})
	}
}

func AdminRefundOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req RefundOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid admin refund request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		if req.Reason == "" {
			req.Reason = "admin refund"
		}
		operatorID := gatewayOperatorID(ctx)
		if err := refundAdminOrder(ctx, svcCtx, nil, req, operatorID); err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s reason:%s", req.OrderID, adminAuditReasonInvalidStatus))
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditOrderRefunded, fmt.Sprintf("order:%s operator:%d reason:%s", req.OrderID, operatorID, req.Reason))
		ok(ctx, c, RefundOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Refunded)})
	}
}

func adminOrderQueryFromRequest(c *app.RequestContext) (AdminOrderListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return AdminOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	return AdminOrderListReq{
		Page:        normalizePage(page),
		PageSize:    normalizeAdminPageSize(pageSize),
		MerchantID:  merchantID,
		UserID:      userID,
		ProductID:   productID,
		Status:      status,
		ProductName: strings.TrimSpace(c.Query("product_name")),
		CreatedFrom: strings.TrimSpace(c.Query("created_from")),
		CreatedTo:   strings.TrimSpace(c.Query("created_to")),
		OrderID:     strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func loadAdminOrders(ctx context.Context, db *sql.DB, req AdminOrderListReq) (AdminOrderListResp, error) {
	where := "1=1"
	args := []any{}
	if req.Status >= 0 {
		where += " AND o.status = ?"
		args = append(args, req.Status)
	}
	if req.UserID > 0 {
		where += " AND o.user_id = ?"
		args = append(args, req.UserID)
	}
	if req.MerchantID > 0 {
		where += " AND o.merchant_id = ?"
		args = append(args, req.MerchantID)
	}
	if req.ProductID > 0 {
		where += " AND o.product_id = ?"
		args = append(args, req.ProductID)
	}
	if req.ProductName != "" {
		where += " AND s.product_name LIKE ?"
		args = append(args, "%"+req.ProductName+"%")
	}
	if req.CreatedFrom != "" {
		where += " AND o.create_time >= ?"
		args = append(args, normalizeAdminDateTimeLower(req.CreatedFrom))
	}
	if req.CreatedTo != "" {
		where += " AND o.create_time <= ?"
		args = append(args, normalizeAdminDateTimeUpper(req.CreatedTo))
	}
	if req.OrderID != "" {
		where += " AND o.id = ?"
		args = append(args, req.OrderID)
	}

	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders o LEFT JOIN order_price_snapshot s ON s.order_id = o.id WHERE "+where, args...).Scan(&total); err != nil {
		return AdminOrderListResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT o.id,
       o.user_id,
       o.merchant_id,
       COALESCE(m.name, ''),
       o.product_id,
       COALESCE(s.product_name, ''),
       o.amount,
       o.status,
       COALESCE(s.payable_amount_fen, 0),
       DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s')
FROM orders o
LEFT JOIN order_price_snapshot s ON s.order_id = o.id
LEFT JOIN merchant m ON m.id = o.merchant_id
WHERE `+where+`
ORDER BY o.create_time DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AdminOrderListResp{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminOrderItem, 0)
	for rows.Next() {
		var item AdminOrderItem
		if err := rows.Scan(&item.OrderID, &item.UserID, &item.MerchantID, &item.MerchantName, &item.ProductID, &item.ProductName, &item.Amount, &item.Status, &item.PayableAmountFen, &item.CreateTime); err != nil {
			return AdminOrderListResp{}, err
		}
		item.StatusText = orderstatus.Text(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminOrderListResp{}, err
	}
	return AdminOrderListResp{Items: items, Total: total}, nil
}

func loadAdminOrderDetail(ctx context.Context, svcCtx *svc.ServiceContext, orderID string) (OrderDetailResp, error) {
	db, err := orderDB(svcCtx)
	if err != nil {
		return OrderDetailResp{}, err
	}
	var userID int64
	if err := db.QueryRowContext(ctx, "SELECT user_id FROM orders WHERE id = ? LIMIT 1", orderID).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrderDetailResp{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return OrderDetailResp{}, err
	}
	return loadUserOrderDetail(ctx, svcCtx, orderID, userID)
}

func loadAdminOrderStatusLogs(ctx context.Context, db *sql.DB, orderID string) (AdminOrderStatusLogResp, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,
       order_id,
       from_status,
       to_status,
       operator_id,
       remark,
       DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s')
FROM order_status_log
WHERE order_id = ?
ORDER BY id ASC`, orderID)
	if err != nil {
		return AdminOrderStatusLogResp{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminOrderStatusLogItem, 0)
	for rows.Next() {
		var item AdminOrderStatusLogItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.FromStatus, &item.ToStatus, &item.OperatorID, &item.Remark, &item.CreateTime); err != nil {
			return AdminOrderStatusLogResp{}, err
		}
		item.FromStatusText = orderstatus.Text(item.FromStatus)
		item.ToStatusText = orderstatus.Text(item.ToStatus)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminOrderStatusLogResp{}, err
	}
	return AdminOrderStatusLogResp{Items: items}, nil
}

func refundAdminOrder(ctx context.Context, svcCtx *svc.ServiceContext, _ *sql.DB, req RefundOrderReq, operatorID int64) error {
	requestID := tracectx.RequestIDFrom(ctx)
	if requestID == "" {
		requestID = req.OrderID + ":admin-refund"
	}
	requested, err := svcCtx.OrderRpc.RequestRefund(ctx, &orderpb.RequestRefundReq{
		OrderId: req.OrderID, RequesterId: operatorID, RequesterRole: "admin",
		Reason: req.Reason, RequestId: requestID,
	})
	if err != nil {
		return err
	}
	_, err = svcCtx.OrderRpc.AuditRefund(ctx, &orderpb.AuditRefundReq{
		RefundId: requested.GetRefundId(), OperatorId: operatorID, Approve: true,
		Remark: req.Reason, RequestId: requestID,
	})
	return err
}

func gatewayOperatorID(ctx context.Context) int64 {
	if identity, ok := authctx.IdentityFrom(ctx); ok && identity.UserID > 0 {
		return identity.UserID
	}
	return 0
}

func normalizeAdminDateTimeLower(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return value + " 00:00:00"
	}
	return value
}

func normalizeAdminDateTimeUpper(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return value + " 23:59:59"
	}
	return value
}

func normalizeAdminPageSize(pageSize int64) int64 {
	if pageSize <= 0 || pageSize > 100 {
		return 20
	}
	return pageSize
}
