package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/svc"
)

func newAuthTestSvc() *svc.ServiceContext {
	return svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
		RefreshCookieName:      "fm_refresh_token",
	})
}

func mustJSON(t *testing.T, body any) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewReader(raw)
}

func TestLoginHandler_SetsRefreshCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", mustJSON(t, map[string]any{
		"user_id":  1001,
		"password": "pwd",
	}))
	req.Header.Set("Content-Type", "application/json")

	LoginHandler(newAuthTestSvc()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "fm_refresh_token=") {
		t.Fatalf("expected refresh cookie, got %q", cookie)
	}
}

func TestRegisterHandler_RegistersUserAndSetsRefreshCookie(t *testing.T) {
	svcCtx := newAuthTestSvc()

	sendCodeRec := httptest.NewRecorder()
	sendCodeReq := httptest.NewRequest(http.MethodPost, "/api/auth/code/send", mustJSON(t, map[string]any{
		"phone": "13800138000",
		"scene": "register",
	}))
	sendCodeReq.Header.Set("Content-Type", "application/json")
	SendCodeHandler(svcCtx).ServeHTTP(sendCodeRec, sendCodeReq)
	if sendCodeRec.Code != http.StatusOK {
		t.Fatalf("send code status: %d", sendCodeRec.Code)
	}

	var codeResp map[string]any
	if err := json.Unmarshal(sendCodeRec.Body.Bytes(), &codeResp); err != nil {
		t.Fatalf("decode code resp: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", mustJSON(t, map[string]any{
		"phone":        "13800138000",
		"password":     "mall-pass-123",
		"code":         codeResp["debug_code"],
		"display_name": "测试新用户",
	}))
	req.Header.Set("Content-Type", "application/json")
	RegisterHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "fm_refresh_token=") {
		t.Fatalf("expected refresh cookie, got %q", cookie)
	}
}

func TestRefreshHandler_RotatesRefreshCookie(t *testing.T) {
	svcCtx := newAuthTestSvc()

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", mustJSON(t, map[string]any{
		"user_id":  1001,
		"password": "pwd",
	}))
	loginReq.Header.Set("Content-Type", "application/json")
	LoginHandler(svcCtx).ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status: %d", loginRec.Code)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.Header.Set("Cookie", loginRec.Header().Get("Set-Cookie"))
	RefreshHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "fm_refresh_token=") {
		t.Fatalf("expected rotated refresh cookie, got %q", cookie)
	}
}

func TestLoginCodeHandler_SetsRefreshCookie(t *testing.T) {
	svcCtx := newAuthTestSvc()

	sendCodeRec := httptest.NewRecorder()
	sendCodeReq := httptest.NewRequest(http.MethodPost, "/api/auth/code/send", mustJSON(t, map[string]any{
		"phone": "13800000001",
		"scene": "login",
	}))
	sendCodeReq.Header.Set("Content-Type", "application/json")
	SendCodeHandler(svcCtx).ServeHTTP(sendCodeRec, sendCodeReq)
	if sendCodeRec.Code != http.StatusOK {
		t.Fatalf("send code status: %d", sendCodeRec.Code)
	}

	var codeResp map[string]any
	if err := json.Unmarshal(sendCodeRec.Body.Bytes(), &codeResp); err != nil {
		t.Fatalf("decode code resp: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/code", mustJSON(t, map[string]any{
		"phone": "13800000001",
		"code":  codeResp["debug_code"],
	}))
	req.Header.Set("Content-Type", "application/json")
	LoginCodeHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "fm_refresh_token=") {
		t.Fatalf("expected refresh cookie, got %q", cookie)
	}
}

func TestLogoutHandler_ClearsRefreshCookie(t *testing.T) {
	svcCtx := newAuthTestSvc()

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", mustJSON(t, map[string]any{
		"user_id":  1001,
		"password": "pwd",
	}))
	loginReq.Header.Set("Content-Type", "application/json")
	LoginHandler(svcCtx).ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status: %d", loginRec.Code)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Cookie", loginRec.Header().Get("Set-Cookie"))
	LogoutHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	cookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "fm_refresh_token=") {
		t.Fatalf("expected cleared refresh cookie, got %q", cookie)
	}
	if !strings.Contains(cookie, "Expires=Thu, 01 Jan 1970 00:00:00 GMT") && !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("expected cleared refresh cookie, got %q", cookie)
	}
}

func TestLogoutAllHandler_ClearsRefreshCookie(t *testing.T) {
	svcCtx := newAuthTestSvc()

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", mustJSON(t, map[string]any{
		"user_id":  1001,
		"password": "pwd",
	}))
	loginReq.Header.Set("Content-Type", "application/json")
	LoginHandler(svcCtx).ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status: %d", loginRec.Code)
	}

	var loginResp map[string]any
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login resp: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout-all", nil)
	req.Header.Set("Cookie", loginRec.Header().Get("Set-Cookie"))
	req = req.WithContext(context.WithValue(req.Context(), "user_id", int64(1001))) //nolint:staticcheck // go-zero JWT stores claims under string keys.
	LogoutAllHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "fm_refresh_token=") {
		t.Fatalf("expected cleared cookie, got %q", cookie)
	}
}

func TestResetPasswordHandler_ChangesPasswordAndInvalidatesSession(t *testing.T) {
	svcCtx := newAuthTestSvc()

	sendCodeRec := httptest.NewRecorder()
	sendCodeReq := httptest.NewRequest(http.MethodPost, "/api/auth/password/forgot", mustJSON(t, map[string]any{
		"phone": "13800000001",
	}))
	sendCodeReq.Header.Set("Content-Type", "application/json")
	ForgotPasswordHandler(svcCtx).ServeHTTP(sendCodeRec, sendCodeReq)
	if sendCodeRec.Code != http.StatusOK {
		t.Fatalf("forgot password status: %d body=%s", sendCodeRec.Code, sendCodeRec.Body.String())
	}

	var codeResp map[string]any
	if err := json.Unmarshal(sendCodeRec.Body.Bytes(), &codeResp); err != nil {
		t.Fatalf("decode forgot-password resp: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password/reset", mustJSON(t, map[string]any{
		"phone":        "13800000001",
		"code":         codeResp["debug_code"],
		"new_password": "new-pass-456",
	}))
	req.Header.Set("Content-Type", "application/json")
	ResetPasswordHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}
