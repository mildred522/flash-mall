package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AuthProxyHandler(svcCtx *svc.ServiceContext, targetPath string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
		if baseURL == "" {
			fail(ctx, c, consts.StatusServiceUnavailable, apperror.New(apperror.CodeInternal, "auth service is not configured"))
			return
		}
		body, err := c.Body()
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid proxy request body"))
			return
		}
		upstreamURL := baseURL + targetPath
		if rawQuery := string(c.QueryArgs().QueryString()); rawQuery != "" {
			upstreamURL += "?" + rawQuery
		}
		req, err := http.NewRequestWithContext(ctx, string(c.Method()), upstreamURL, bytes.NewReader(body))
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "build auth proxy request failed", err))
			return
		}
		req.ContentLength = int64(len(body))
		copyProxyRequestHeaders(c, req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fail(ctx, c, consts.StatusServiceUnavailable, apperror.Wrap(apperror.CodeInternal, "auth service unavailable", err))
			return
		}
		defer func() { _ = resp.Body.Close() }()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "read auth proxy response failed", err))
			return
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/json"
		}
		c.Data(resp.StatusCode, contentType, respBody)
	}
}

func copyProxyRequestHeaders(c *app.RequestContext, req *http.Request) {
	c.Request.Header.VisitAll(func(key, value []byte) {
		headerName := string(key)
		if !proxyHeaderAllowed(headerName) {
			return
		}
		req.Header.Add(headerName, string(value))
	})
}

func proxyHeaderAllowed(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return false
	default:
		return true
	}
}
