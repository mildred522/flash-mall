package handler

import (
	"fmt"
	"net/http"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/order/rpc/orderclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func AdminOrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminOrderListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		offset := (req.Page - 1) * req.PageSize

		where := "1=1"
		args := []interface{}{}
		if req.Status >= 0 {
			where += " AND o.status = ?"
			args = append(args, req.Status)
		}
		if req.UserId > 0 {
			where += " AND o.user_id = ?"
			args = append(args, req.UserId)
		}
		if req.OrderId != "" {
			where += " AND o.id = ?"
			args = append(args, req.OrderId)
		}

		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM orders o WHERE %s", where)
		var total int64
		_ = db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)

		query := fmt.Sprintf(
			`SELECT o.id, o.user_id, o.product_id, COALESCE(s.product_name, ''), o.amount, o.status, COALESCE(o.payable_amount_fen, 0), COALESCE(o.create_time, '')
			 FROM orders o LEFT JOIN order_price_snapshot s ON s.order_id = o.id
			 WHERE %s ORDER BY o.create_time DESC LIMIT ? OFFSET ?`, where)
		args = append(args, req.PageSize, offset)

		rows, err := db.QueryContext(r.Context(), query, args...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin order list query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer rows.Close()

		items := make([]types.AdminOrderListItem, 0)
		for rows.Next() {
			var item types.AdminOrderListItem
			if err := rows.Scan(&item.OrderId, &item.UserId, &item.ProductId, &item.ProductName, &item.Amount, &item.Status, &item.PayableAmountFen, &item.CreateTime); err != nil {
				continue
			}
			item.StatusText = orderStatusText(item.Status)
			items = append(items, item)
		}

		httpx.OkJsonCtx(r.Context(), w, types.AdminOrderListResp{Items: items, Total: total})
	}
}

func AdminOrderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderId := r.URL.Query().Get("order_id")
		if orderId == "" {
			httpx.OkJsonCtx(r.Context(), w, map[string]any{"error": "order_id required"})
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var item types.OrderDetailResp
		err = db.QueryRowContext(r.Context(),
			`SELECT o.id, o.product_id, COALESCE(s.product_name,''), o.amount, o.status,
			        COALESCE(s.origin_unit_price_fen,0), COALESCE(s.sale_unit_price_fen,0),
			        COALESCE(s.payable_amount_fen,0), COALESCE(s.discount_amount_fen,0),
			        COALESCE(s.promotion_type,''), COALESCE(s.promotion_tag,''),
			        COALESCE(p.id,''), COALESCE(p.status,0), COALESCE(o.create_time,'')
			 FROM orders o
			 LEFT JOIN order_price_snapshot s ON s.order_id = o.id
			 LEFT JOIN payment_order p ON p.order_id = o.id
			 WHERE o.id = ?`, orderId,
		).Scan(&item.OrderId, &item.ProductId, &item.ProductName, &item.Amount, &item.Status,
			&item.OriginUnitPriceFen, &item.SaleUnitPriceFen, &item.PayableAmountFen, &item.DiscountAmountFen,
			&item.PromotionType, &item.PromotionTag, &item.PaymentOrderId, &item.PaymentStatus, &item.CreateTime)
		if err != nil {
			httpx.OkJsonCtx(r.Context(), w, map[string]any{"error": "order not found"})
			return
		}
		item.StatusText = orderStatusText(item.Status)
		httpx.OkJsonCtx(r.Context(), w, item)
	}
}

func AdminShipOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ShipOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		result, err := db.ExecContext(r.Context(),
			"UPDATE orders SET status = 3, shipped_at = NOW() WHERE id = ? AND status = 1", req.OrderId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			httpx.OkJsonCtx(r.Context(), w, map[string]any{"error": "order not in paid status"})
			return
		}
		_, _ = db.ExecContext(r.Context(),
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, 1, 3, 0, 'admin shipped')", req.OrderId)

		httpx.OkJsonCtx(r.Context(), w, types.ShipOrderResp{OrderId: req.OrderId, Status: "shipped"})
	}
}

func AdminRefundOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RefundOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.OrderId == "" {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.InvalidArgument, "order_id is required"))
			return
		}
		if svcCtx.OrderRpc == nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "order rpc not configured"))
			return
		}
		resp, err := svcCtx.OrderRpc.ApproveRefund(r.Context(), &orderclient.LifecycleOrderReq{
			OrderId:      req.OrderId,
			OperatorId:   0,
			OperatorRole: "admin",
			Reason:       req.Reason,
			RequestId:    req.OrderId + ":refund-approve",
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, types.RefundOrderResp{OrderId: resp.OrderId, Status: resp.StatusText})
	}
}
