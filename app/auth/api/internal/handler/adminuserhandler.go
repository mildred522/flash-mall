package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/logic/auth"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminUserListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := auth.NewAdminUserListLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserList()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
