package handler

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if trustedProxyPeer(r.RemoteAddr) {
		if xff := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); xff != "" {
			return xff
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func trustedProxyPeer(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate()
}
