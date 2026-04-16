package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_TrustsForwardedForFromPrivatePeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("unexpected ip: %s", got)
	}
}

func TestClientIP_IgnoresForwardedForFromPublicPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.9" {
		t.Fatalf("unexpected ip: %s", got)
	}
}
