package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/logic/auth"
	"flash-mall/app/auth/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SecurityEventsRecentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := auth.NewSecurityEventsLogic(r.Context(), svcCtx).ListRecent()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
