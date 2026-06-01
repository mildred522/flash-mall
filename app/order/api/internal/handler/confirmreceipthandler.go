package handler

import (
	"net/http"

	"flash-mall/app/order/api/internal/logic"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ConfirmReceiptHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfirmReceiptReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
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

		l := logic.NewConfirmReceiptLogic(r.Context(), svcCtx)
		resp, err := l.ConfirmReceipt(&req, identity.UserID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
