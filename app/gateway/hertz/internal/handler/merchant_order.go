package handler

import (
	"context"
	"database/sql"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func MerchantOrderListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, merchantID, ready := merchantOrderDB(ctx, c, svcCtx, identity)
		if !ready {
			return
		}
		query, err := merchantOrderQueryFromRequest(c, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		resp, err := loadMerchantOrders(ctx, db, query)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant order query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantRefundListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, merchantID, ready := merchantOrderDB(ctx, c, svcCtx, identity)
		if !ready {
			return
		}
		query, err := merchantRefundQueryFromRequest(c, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		resp, err := loadMerchantRefunds(ctx, db, query)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant refund query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantShipOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		_, merchantID, ready := merchantOrderDB(ctx, c, svcCtx, identity)
		if !ready {
			return
		}

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
		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.ShipMerchant(ctx, ports.ShipMerchantOrderCommand{OrderID: req.OrderID, MerchantID: merchantID})
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, MerchantShipOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Shipped)})
	}
}

func merchantOrderDB(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, identity authctx.Identity) (*sql.DB, int64, bool) {
	db, err := orderDB(svcCtx)
	if err != nil {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
		return nil, 0, false
	}
	merchantID, err := selectedMerchantID(ctx, db, identity)
	if err != nil {
		fail(ctx, c, consts.StatusForbidden, err)
		return nil, 0, false
	}
	return db, merchantID, true
}

func merchantOrderQueryFromRequest(c *app.RequestContext, merchantID int64) (MerchantOrderListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 50)
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	return MerchantOrderListReq{
		MerchantID: merchantID,
		Page:       normalizePage(page),
		PageSize:   normalizePageSize(pageSize),
		Status:     status,
		UserID:     userID,
		ProductID:  productID,
		OrderID:    strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func merchantRefundQueryFromRequest(c *app.RequestContext, merchantID int64) (MerchantRefundListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 50)
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	return MerchantRefundListReq{
		MerchantID: merchantID,
		Page:       normalizePage(page),
		PageSize:   normalizePageSize(pageSize),
		Status:     status,
		UserID:     userID,
		OrderID:    strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func loadMerchantOrders(ctx context.Context, db *sql.DB, req MerchantOrderListReq) (MerchantOrderListResp, error) {
	where := "o.merchant_id = ?"
	args := []any{req.MerchantID}
	if req.Status >= 0 {
		where += " AND o.status = ?"
		args = append(args, req.Status)
	}
	if req.UserID > 0 {
		where += " AND o.user_id = ?"
		args = append(args, req.UserID)
	}
	if req.ProductID > 0 {
		where += " AND o.product_id = ?"
		args = append(args, req.ProductID)
	}
	if req.OrderID != "" {
		where += " AND o.id = ?"
		args = append(args, req.OrderID)
	}

	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders o WHERE "+where, args...).Scan(&total); err != nil {
		return MerchantOrderListResp{}, err
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
		return MerchantOrderListResp{}, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]MerchantOrderItem, 0)
	for rows.Next() {
		var item MerchantOrderItem
		if err := rows.Scan(&item.OrderID, &item.UserID, &item.MerchantID, &item.MerchantName, &item.ProductID, &item.ProductName, &item.Amount, &item.Status, &item.PayableAmountFen, &item.CreateTime); err != nil {
			return MerchantOrderListResp{}, err
		}
		item.StatusText = orderstatus.Text(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MerchantOrderListResp{}, err
	}
	return MerchantOrderListResp{Items: items, Total: total}, nil
}

func loadMerchantRefunds(ctx context.Context, db *sql.DB, req MerchantRefundListReq) (MerchantRefundListResp, error) {
	where := "r.merchant_id = ?"
	args := []any{req.MerchantID}
	if req.Status >= 0 {
		where += " AND r.status = ?"
		args = append(args, req.Status)
	}
	if req.UserID > 0 {
		where += " AND r.user_id = ?"
		args = append(args, req.UserID)
	}
	if req.OrderID != "" {
		where += " AND r.order_id = ?"
		args = append(args, req.OrderID)
	}

	var total int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM refund_order r WHERE "+where, args...).Scan(&total); err != nil {
		return MerchantRefundListResp{}, err
	}
	queryArgs := append(append([]any{}, args...), req.PageSize, (req.Page-1)*req.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT r.id,
       r.order_id,
       r.payment_order_id,
       r.user_id,
       r.merchant_id,
       COALESCE(m.name, ''),
       r.product_id,
       r.refund_amount_fen,
       r.status,
       r.reason,
       r.audit_remark,
       r.operator_id,
       DATE_FORMAT(r.request_time, '%Y-%m-%d %H:%i:%s'),
       COALESCE(DATE_FORMAT(r.audit_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(r.finish_time, '%Y-%m-%d %H:%i:%s'), '')
FROM refund_order r
LEFT JOIN merchant m ON m.id = r.merchant_id
WHERE `+where+`
ORDER BY r.create_time DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return MerchantRefundListResp{}, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]MerchantRefundItem, 0)
	for rows.Next() {
		var item MerchantRefundItem
		if err := rows.Scan(&item.RefundID, &item.OrderID, &item.PaymentOrderID, &item.UserID, &item.MerchantID, &item.MerchantName, &item.ProductID, &item.RefundAmountFen, &item.Status, &item.Reason, &item.AuditRemark, &item.OperatorID, &item.RequestTime, &item.AuditTime, &item.FinishTime); err != nil {
			return MerchantRefundListResp{}, err
		}
		item.StatusText = refundStatusText(item.Status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MerchantRefundListResp{}, err
	}
	return MerchantRefundListResp{Items: items, Total: total}, nil
}

func refundStatusText(status int64) string {
	switch status {
	case 0:
		return "requested"
	case 1:
		return "approved"
	case 2:
		return "success"
	case 3:
		return "rejected"
	case 4:
		return "failed"
	default:
		return "unknown"
	}
}

func normalizePage(page int64) int64 {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int64) int64 {
	if pageSize <= 0 || pageSize > 100 {
		return 50
	}
	return pageSize
}
