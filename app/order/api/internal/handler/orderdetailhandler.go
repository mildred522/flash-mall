package handler

import (
	"net/http"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func OrderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.SessionValidator != nil {
			if err := svcCtx.SessionValidator.Validate(r.Context(), r.Header.Get("Authorization")); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.InvalidArgument, "order_id is required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "db connection failed"))
			return
		}

		var resp types.OrderDetailResp
		err = db.QueryRowContext(r.Context(), `
SELECT o.id, o.product_id, o.amount, o.status, o.create_time,
       s.product_name, s.origin_unit_price_fen, s.sale_unit_price_fen,
       s.payable_amount_fen, s.discount_amount_fen,
       s.promotion_type, s.promotion_tag,
       p.id, p.status
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ? AND o.user_id = ?
LIMIT 1`, orderID, identity.UserID).Scan(
			&resp.OrderId, &resp.ProductId, &resp.Amount, &resp.Status, &resp.CreateTime,
			&resp.ProductName, &resp.OriginUnitPriceFen, &resp.SaleUnitPriceFen,
			&resp.PayableAmountFen, &resp.DiscountAmountFen,
			&resp.PromotionType, &resp.PromotionTag,
			&resp.PaymentOrderId, &resp.PaymentStatus,
		)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.NotFound, "order not found"))
			return
		}

		resp.StatusText = orderStatusText(resp.Status)
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
