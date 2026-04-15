package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShopUIIncludesAuthAndStorefrontAnchors(t *testing.T) {
	req := httptest.NewRequest("GET", "/shop", nil)
	rec := httptest.NewRecorder()

	ShopUIHandler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, needle := range []string{
		"Flash Mall",
		"developer-console",
		"auth-modal",
		"auth-tab-password",
		"auth-tab-register",
		"auth-tab-code",
		"auth-tab-reset",
		"send-code-action",
		"reset-password-action",
		"/api/auth/register",
		"/api/auth/login/code",
		"/api/auth/refresh",
		"/api/auth/logout",
		"/api/auth/password/forgot",
		"/api/auth/password/reset",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected shop UI to contain %q", needle)
		}
	}
}

func TestHomeUIIncludesEntryAnchors(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	HomeUIHandler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, needle := range []string{
		"Flash Mall",
		"entry-shop",
		"entry-campaign",
		"/shop",
		"/shop#campaign-strip",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected home UI to contain %q", needle)
		}
	}
}

func TestDebugUIIncludesDeveloperAnchors(t *testing.T) {
	req := httptest.NewRequest("GET", "/debug", nil)
	rec := httptest.NewRecorder()

	DebugUIHandler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, needle := range []string{
		"Flash Mall",
		"/shop",
		"/api/system/health",
		"run-health",
		"debug-log",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected debug UI to contain %q", needle)
		}
	}
}
