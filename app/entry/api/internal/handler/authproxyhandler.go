package handler

import (
	"io"
	"net/http"
	"strings"

	"flash-mall/app/entry/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func AuthProxyHandler(svcCtx *svc.ServiceContext, targetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
		if baseURL == "" {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unavailable, "auth service not configured"))
			return
		}

		var bodyReader io.Reader
		if r.Body != nil {
			bodyReader = r.Body
		}

		upstreamURL := baseURL + targetPath
		if r.URL.RawQuery != "" {
			upstreamURL += "?" + r.URL.RawQuery
		}
		upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bodyReader)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "build auth proxy request failed"))
			return
		}
		upstreamReq.ContentLength = r.ContentLength
		copyHeaders(upstreamReq.Header, r.Header)

		upstreamResp, err := http.DefaultClient.Do(upstreamReq)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unavailable, "auth service unavailable"))
			return
		}
		defer func() { _ = upstreamResp.Body.Close() }()

		copyHeaders(w.Header(), upstreamResp.Header)
		w.WriteHeader(upstreamResp.StatusCode)
		_, _ = io.Copy(w, upstreamResp.Body)
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if !proxyHeaderAllowed(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func proxyHeaderAllowed(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return false
	default:
		return true
	}
}
