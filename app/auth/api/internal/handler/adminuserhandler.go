package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"flash-mall/app/auth/api/internal/logic/auth"
	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminUserListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdminRequest(r) {
			writeAdminDenied(w)
			return
		}
		var req struct {
			Page     int64  `form:"page,optional,default=1"`       //nolint:staticcheck // go-zero httpx.Parse uses optional in tags.
			PageSize int64  `form:"page_size,optional,default=20"` //nolint:staticcheck // go-zero httpx.Parse uses optional in tags.
			Status   int64  `form:"status,optional"`               //nolint:staticcheck // go-zero httpx.Parse uses optional in tags.
			Role     string `form:"role,optional"`                 //nolint:staticcheck // go-zero httpx.Parse uses optional in tags.
			Keyword  string `form:"keyword,optional"`              //nolint:staticcheck // go-zero httpx.Parse uses optional in tags.
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := auth.NewAdminUserListLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserList(req.Page, req.PageSize, req.Status, req.Role, req.Keyword)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func AdminUserDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdminRequest(r) {
			writeAdminDenied(w)
			return
		}
		var req struct {
			UserId int64 `form:"user_id"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := auth.NewAdminUserDetailLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserDetail(req.UserId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AdminUserStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdminRequest(r) {
			writeAdminDenied(w)
			return
		}
		var req struct {
			UserId int64 `json:"user_id"`
			Status int64 `json:"status"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Status == 2 && req.UserId == currentUserID(r) {
			auth.NewAdminUserStatusLogic(r.Context(), svcCtx).RecordFailure(req.UserId, req.Status, auth.AdminAuditReasonSelfDisableBlocked)
			writeAdminJSONError(w, http.StatusConflict, "cannot disable current admin")
			return
		}
		l := auth.NewAdminUserStatusLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserStatus(req.UserId, req.Status)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func isAdminRequest(r *http.Request) bool {
	role, _ := r.Context().Value("role").(string)
	return role == "admin"
}

func writeAdminDenied(w http.ResponseWriter) {
	writeAdminJSONError(w, http.StatusForbidden, "admin access required")
}

func writeAdminJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func currentUserID(r *http.Request) int64 {
	switch value := r.Context().Value("user_id").(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case string:
		id, _ := strconv.ParseInt(value, 10, 64)
		return id
	default:
		return 0
	}
}
