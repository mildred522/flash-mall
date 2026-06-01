package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func TestShopUIReturnsHTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/shop", nil)
	rec := httptest.NewRecorder()

	ShopUIHandler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Flash Mall") {
		t.Fatalf("expected shop UI to contain 'Flash Mall'")
	}
	if !containsIgnoreCase(body, "<!DOCTYPE html>") {
		t.Fatalf("expected valid HTML")
	}
}

func TestHomeUIIncludesEntryAnchors(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	HomeUIHandler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Flash Mall") {
		t.Fatalf("expected home UI to contain 'Flash Mall'")
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
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected debug UI to contain %q", needle)
		}
	}
}

func TestAdminUIReturnsHTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/admin", nil)
	rec := httptest.NewRecorder()

	AdminUIHandler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsIgnoreCase(body, "Flash Mall") {
		t.Fatalf("expected admin UI to contain 'Flash Mall'")
	}
}

func TestMonitorUIReturnsHTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/monitor", nil)
	rec := httptest.NewRecorder()

	MonitorUIHandler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsIgnoreCase(body, "Flash Mall") {
		t.Fatalf("expected monitor UI to contain 'Flash Mall'")
	}
	if !strings.Contains(body, "/api/system/health") {
		t.Fatalf("expected monitor UI to reference health endpoint")
	}
	if !strings.Contains(body, "/metrics") {
		t.Fatalf("expected monitor UI to reference metrics endpoint")
	}
}
