package handler

import (
	"net/http"

	"flash-mall/app/order/api/internal/svc"
)

func AdminUserListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return AuthProxyHandler(svcCtx, "/api/admin/users")
}

func AdminUserDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return AuthProxyHandler(svcCtx, "/api/admin/users/detail")
}
