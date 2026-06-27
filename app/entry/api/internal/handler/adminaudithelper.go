package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/entry/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type adminAuditPayload struct {
	EventType string `json:"event_type"`
	Result    string `json:"result"`
	Subject   string `json:"subject,omitempty"`
}

func recordAdminAuditEvent(r *http.Request, svcCtx *svc.ServiceContext, eventType, subject string) {
	recordAdminAuditEventResult(r, svcCtx, eventType, adminAuditResultSuccess, subject)
}

func recordAdminAuditFailure(r *http.Request, svcCtx *svc.ServiceContext, eventType, subject string) {
	recordAdminAuditEventResult(r, svcCtx, eventType, adminAuditResultFail, subject)
}

func recordAdminAuditEventResult(r *http.Request, svcCtx *svc.ServiceContext, eventType, result, subject string) {
	baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
	eventType = strings.TrimSpace(eventType)
	if baseURL == "" || eventType == "" {
		return
	}

	authz := r.Header.Get("Authorization")
	userAgent := r.UserAgent()
	forwardedFor := r.Header.Get("X-Forwarded-For")
	realIP := r.Header.Get("X-Real-IP")
	subject = strings.TrimSpace(subject)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		payload, err := json.Marshal(adminAuditPayload{
			EventType: eventType,
			Result:    result,
			Subject:   subject,
		})
		if err != nil {
			logx.Errorf("admin audit marshal failed: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/admin/security/events/record", bytes.NewReader(payload))
		if err != nil {
			logx.Errorf("admin audit request build failed: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
		if forwardedFor != "" {
			req.Header.Set("X-Forwarded-For", forwardedFor)
		}
		if realIP != "" {
			req.Header.Set("X-Real-IP", realIP)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logx.Errorf("admin audit request failed: %v", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= http.StatusMultipleChoices {
			logx.Errorf("admin audit request returned status=%d event_type=%s", resp.StatusCode, eventType)
		}
	}()
}
