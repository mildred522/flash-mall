package handler

import (
	"net/http"
	"strings"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func UserAddressListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		rows, err := db.QueryContext(r.Context(),
			`SELECT id, receiver_name, receiver_phone, province, city, district, detail, is_default
			   FROM mall_auth.user_address
			  WHERE user_id = ? AND status = 1
			  ORDER BY is_default DESC, id DESC`, identity.UserID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		items := make([]types.UserAddressItem, 0)
		for rows.Next() {
			var item types.UserAddressItem
			var isDefault int64
			if err := rows.Scan(&item.AddressId, &item.ReceiverName, &item.ReceiverPhone, &item.Province,
				&item.City, &item.District, &item.Detail, &isDefault); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.IsDefault = isDefault == 1
			items = append(items, item)
		}
		httpx.OkJsonCtx(r.Context(), w, types.UserAddressListResp{Items: items})
	}
}

func UserAddressUpsertHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UserAddressUpsertReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.ReceiverName = strings.TrimSpace(req.ReceiverName)
		req.ReceiverPhone = strings.TrimSpace(req.ReceiverPhone)
		req.Detail = strings.TrimSpace(req.Detail)
		if req.ReceiverName == "" || req.ReceiverPhone == "" || req.Detail == "" {
			writeBadRequest(w, "receiver_name, receiver_phone and detail required")
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
		if req.IsDefault {
			if _, err := tx.ExecContext(r.Context(), "UPDATE mall_auth.user_address SET is_default = 0 WHERE user_id = ?", identity.UserID); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		isDefault := int64(0)
		if req.IsDefault {
			isDefault = 1
		}
		addressID := req.AddressId
		if req.AddressId > 0 {
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE mall_auth.user_address
				    SET receiver_name=?, receiver_phone=?, province=?, city=?, district=?, detail=?, is_default=?
				  WHERE id=? AND user_id=? AND status=1`,
				req.ReceiverName, req.ReceiverPhone, req.Province, req.City, req.District, req.Detail, isDefault, req.AddressId, identity.UserID); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		} else {
			result, err := tx.ExecContext(r.Context(),
				`INSERT INTO mall_auth.user_address
				  (user_id, receiver_name, receiver_phone, province, city, district, detail, is_default)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				identity.UserID, req.ReceiverName, req.ReceiverPhone, req.Province, req.City, req.District, req.Detail, isDefault)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			addressID, _ = result.LastInsertId()
		}
		if err := tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, types.UserAddressUpsertResp{AddressId: addressID})
	}
}
