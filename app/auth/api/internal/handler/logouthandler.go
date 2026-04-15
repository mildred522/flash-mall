package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/logic/auth"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func LogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(svcCtx.Config.RefreshCookieName)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, "missing refresh token"))
			return
		}

		l := auth.NewLogoutLogic(r.Context(), svcCtx)
		if err := l.Logout(cookie.Value); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		clearRefreshCookie(w, svcCtx)
		httpx.OkJsonCtx(r.Context(), w, &types.LogoutResp{Success: true})
	}
}
