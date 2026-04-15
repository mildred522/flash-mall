package handler

import (
	"net/http"
	"time"

	"flash-mall/app/auth/api/internal/svc"
)

func writeRefreshCookie(w http.ResponseWriter, svcCtx *svc.ServiceContext, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     svcCtx.Config.RefreshCookieName,
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Duration(svcCtx.Config.RefreshTokenTTLSeconds) * time.Second),
		MaxAge:   int(svcCtx.Config.RefreshTokenTTLSeconds),
	})
}

func clearRefreshCookie(w http.ResponseWriter, svcCtx *svc.ServiceContext) {
	http.SetCookie(w, &http.Cookie{
		Name:     svcCtx.Config.RefreshCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   0,
	})
}
