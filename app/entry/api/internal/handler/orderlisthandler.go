package handler

import (
	"net/http"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func OrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "db connection failed"))
			return
		}

		rows, err := db.QueryContext(r.Context(), `
SELECT o.id, o.product_id, o.amount, o.status, o.create_time,
       s.product_name, s.payable_amount_fen
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
WHERE o.user_id = ?
ORDER BY o.create_time DESC
LIMIT 50`, identity.UserID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "query failed"))
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]types.OrderListItem, 0)
		for rows.Next() {
			var item types.OrderListItem
			if err := rows.Scan(&item.OrderId, &item.ProductId, &item.Amount,
				&item.Status, &item.CreateTime, &item.ProductName, &item.PayableAmountFen); err != nil {
				logx.Errorf("scan order row failed: %v", err)
				httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "scan order row failed"))
				return
			}
			item.StatusText = orderStatusText(item.Status)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			logx.Errorf("iterate order rows failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "iterate order rows failed"))
			return
		}

		httpx.OkJsonCtx(r.Context(), w, types.OrderListResp{Items: items})
	}
}
