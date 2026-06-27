package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/logic/auth"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func LoginCodeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginCodeReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.ClientIP = clientIP(r)
		req.UserAgent = r.UserAgent()

		l := auth.NewLoginCodeLogic(r.Context(), svcCtx)
		resp, err := l.Login(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		writeRefreshCookie(w, svcCtx, resp.RefreshToken)
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
