package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func CancelOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CancelOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.OrderId = strings.TrimSpace(req.OrderId)
		if req.OrderId == "" {
			writeBadRequest(w, "order_id required")
			return
		}
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
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		var currentStatus int64
		var productID int64
		var amount int64
		err = tx.QueryRowContext(r.Context(), "SELECT status, product_id, amount FROM orders WHERE id = ? AND user_id = ? FOR UPDATE", req.OrderId, identity.UserID).Scan(&currentStatus, &productID, &amount)
		if err != nil {
			if err == sql.ErrNoRows {
				writeNotFound(w, "order not found")
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if currentStatus != orderstatus.PendingPayment {
			writeConflict(w, "order cannot be cancelled")
			return
		}

		if _, err := tx.ExecContext(r.Context(), "UPDATE orders SET status = ? WHERE id = ? AND status = ?", orderstatus.Closed, req.OrderId, orderstatus.PendingPayment); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			reason = "user cancel"
		}
		if _, err := tx.ExecContext(r.Context(),
			"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
			req.OrderId, orderstatus.PendingPayment, orderstatus.Closed, identity.UserID, fmt.Sprintf("user cancelled: %s", reason)); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := compensateClosedOrderInventory(r.Context(), svcCtx, productID, amount, req.OrderId); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, types.CancelOrderResp{OrderId: req.OrderId, Status: orderStatusText(orderstatus.Closed)})
	}
}
