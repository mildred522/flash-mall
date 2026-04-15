package handler

import (
	"net/http"

	"flash-mall/app/auth/api/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/login",
			Handler: LoginHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/login/code",
			Handler: LoginCodeHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/register",
			Handler: RegisterHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/refresh",
			Handler: RefreshHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/code/send",
			Handler: SendCodeHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/password/forgot",
			Handler: ForgotPasswordHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/auth/password/reset",
			Handler: ResetPasswordHandler(serverCtx),
		},
	})

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/logout",
				Handler: LogoutHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/auth/logout-all",
				Handler: LogoutAllHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/auth/me",
				Handler: MeHandler(serverCtx),
			},
		},
		rest.WithJwt(serverCtx.Config.JwtAuthSecret),
	)
}
