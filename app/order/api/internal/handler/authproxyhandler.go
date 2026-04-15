package handler

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"flash-mall/app/order/api/internal/svc"
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
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "read auth proxy request body failed"))
				return
			}
			bodyReader = bytes.NewReader(bodyBytes)
		}

		upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, baseURL+targetPath, bodyReader)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Internal, "build auth proxy request failed"))
			return
		}
		copyHeaders(upstreamReq.Header, r.Header)

		upstreamResp, err := http.DefaultClient.Do(upstreamReq)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unavailable, "auth service unavailable"))
			return
		}
		defer upstreamResp.Body.Close()

		copyHeaders(w.Header(), upstreamResp.Header)
		w.WriteHeader(upstreamResp.StatusCode)
		_, _ = io.Copy(w, upstreamResp.Body)
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
