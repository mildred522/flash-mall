package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/entry/api/internal/config"
	"flash-mall/app/entry/api/internal/svc"
)

func TestAuthProxyHandler_ForwardsRequestBodyAndResponse(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		w.Header().Set("Set-Cookie", "refresh_token=test-token; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AuthServiceBaseURL: upstream.URL,
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"user_id":1001}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	AuthProxyHandler(svcCtx, "/api/auth/login").ServeHTTP(rec, req)

	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotPath != "/api/auth/login" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if string(gotBody) != `{"user_id":1001}` {
		t.Fatalf("unexpected body: %s", string(gotBody))
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if gotCookie := rec.Header().Get("Set-Cookie"); gotCookie == "" {
		t.Fatalf("expected set-cookie header to be forwarded")
	}
}

func TestAuthProxyHandler_ForwardsAuthorizationHeader(t *testing.T) {
	var gotAuth string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":1001}`))
	}))
	defer upstream.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AuthServiceBaseURL: upstream.URL,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer test-access-token")
	rec := httptest.NewRecorder()

	AuthProxyHandler(svcCtx, "/api/auth/me").ServeHTTP(rec, req)

	if gotAuth != "Bearer test-access-token" {
		t.Fatalf("unexpected authorization header: %s", gotAuth)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestAuthProxyHandler_ForwardsStreamingRequestBody(t *testing.T) {
	var gotBody []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AuthServiceBaseURL: upstream.URL,
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/code/send", nil)
	req.Body = io.NopCloser(strings.NewReader(`{"phone":"13900001234","scene":"register"}`))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	AuthProxyHandler(svcCtx, "/api/auth/code/send").ServeHTTP(rec, req)

	if string(gotBody) != `{"phone":"13900001234","scene":"register"}` {
		t.Fatalf("unexpected body: %s", string(gotBody))
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestAuthProxyHandler_ForwardsSecurityEventsRoute(t *testing.T) {
	var gotPath string
	var gotAuth string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer upstream.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AuthServiceBaseURL: upstream.URL,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/security/events/recent", nil)
	req.Header.Set("Authorization", "Bearer test-access-token")
	rec := httptest.NewRecorder()

	AuthProxyHandler(svcCtx, "/api/auth/security/events/recent").ServeHTTP(rec, req)

	if gotPath != "/api/auth/security/events/recent" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if gotAuth != "Bearer test-access-token" {
		t.Fatalf("unexpected authorization header: %s", gotAuth)
	}
	if body := rec.Body.String(); body != `{"items":[]}` {
		t.Fatalf("unexpected body: %s", body)
	}
}
