package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/logic/auth"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ResetPasswordHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ResetPasswordReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := auth.NewResetPasswordLogic(r.Context(), svcCtx)
		if err := l.Reset(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		clearRefreshCookie(w, svcCtx)
		httpx.OkJsonCtx(r.Context(), w, &types.LogoutResp{Success: true})
	}
}
