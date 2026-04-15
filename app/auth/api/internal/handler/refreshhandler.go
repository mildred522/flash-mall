package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/logic/auth"
	"flash-mall/app/auth/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RefreshHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(svcCtx.Config.RefreshCookieName)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, "missing refresh token"))
			return
		}

		l := auth.NewRefreshLogic(r.Context(), svcCtx)
		resp, err := l.Refresh(cookie.Value)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		writeRefreshCookie(w, svcCtx, resp.RefreshToken)
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
