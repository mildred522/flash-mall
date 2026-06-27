package auth

import (
	"context"
	"strconv"
	"strings"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SecurityEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSecurityEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecurityEventsLogic {
	return &SecurityEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecurityEventsLogic) ListRecent() (*types.SecurityEventsRecentResp, error) {
	userID, ok := parseUserIDClaim(l.ctx.Value("user_id"))
	if !ok {
		userID, ok = parseUserIDClaim(l.ctx.Value("sub"))
	}
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user subject in jwt")
	}

	limit := int(l.svcCtx.Config.SecurityAuditRecentLimit)
	if limit <= 0 {
		limit = 10
	}

	items, err := l.svcCtx.AuditRecorder.ListRecent(l.ctx, limit*5)
	if err != nil {
		return nil, err
	}

	resp := &types.SecurityEventsRecentResp{
		Items: make([]types.SecurityEventItem, 0, limit),
	}
	for _, item := range items {
		if item.UserID != userID {
			continue
		}
		resp.Items = append(resp.Items, mapSecurityEvent(item))
		if len(resp.Items) >= limit {
			break
		}
	}
	return resp, nil
}

func (l *SecurityEventsLogic) ListAdminRecent(req types.SecurityEventsRecentReq) (*types.SecurityEventsRecentResp, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = int(l.svcCtx.Config.SecurityAuditRecentLimit)
		if limit <= 0 {
			limit = 20
		}
	}
	if limit > 100 {
		limit = 100
	}

	fetchLimit := limit * 5
	if fetchLimit < 50 {
		fetchLimit = 50
	}
	items, err := l.svcCtx.AuditRecorder.ListRecent(l.ctx, fetchLimit)
	if err != nil {
		return nil, err
	}

	resp := &types.SecurityEventsRecentResp{
		Items: make([]types.SecurityEventItem, 0, limit),
	}
	for _, item := range items {
		if req.UserId > 0 && item.UserID != req.UserId {
			continue
		}
		if req.EventType != "" && item.EventType != req.EventType {
			continue
		}
		if req.Result != "" && !securityEventResultMatches(item.Result, req.Result) {
			continue
		}
		if req.Keyword != "" && !securityEventKeywordMatches(item, req.Keyword) {
			continue
		}
		resp.Items = append(resp.Items, mapSecurityEvent(item))
		if len(resp.Items) >= limit {
			break
		}
	}
	return resp, nil
}

func (l *SecurityEventsLogic) RecordAdmin(req types.AdminSecurityEventRecordReq, ip, userAgent string) error {
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		return status.Error(codes.InvalidArgument, "event_type is required")
	}
	result := strings.ToLower(strings.TrimSpace(req.Result))
	if result == "" {
		result = adminAuditResultSuccess
	}
	if result == "failed" {
		result = adminAuditResultFail
	}
	if result != adminAuditResultSuccess && result != adminAuditResultFail {
		return status.Error(codes.InvalidArgument, "result must be success or fail")
	}
	adminID, ok := parseUserIDClaim(l.ctx.Value("user_id"))
	if !ok {
		adminID, ok = parseUserIDClaim(l.ctx.Value("sub"))
	}
	if !ok {
		return status.Error(codes.Unauthenticated, "missing user subject in jwt")
	}
	recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
		EventType:     eventType,
		Result:        result,
		UserID:        adminID,
		IdentityValue: strings.TrimSpace(req.Subject),
		IP:            strings.TrimSpace(ip),
		UserAgent:     strings.TrimSpace(userAgent),
	})
	return nil
}

func securityEventResultMatches(actual, expected string) bool {
	actual = strings.ToLower(strings.TrimSpace(actual))
	expected = strings.ToLower(strings.TrimSpace(expected))
	if actual == expected {
		return true
	}
	return (actual == adminAuditResultFail || actual == "failed") && (expected == adminAuditResultFail || expected == "failed")
}

func securityEventKeywordMatches(item audit.Event, keyword string) bool {
	value := strings.ToLower(strings.TrimSpace(keyword))
	if value == "" {
		return true
	}
	source := strings.ToLower(item.EventType + " " + item.Result + " " + strconv.FormatInt(item.UserID, 10) + " " + item.IdentityValue + " " + item.IP + " " + item.UserAgent)
	return strings.Contains(source, value)
}

func mapSecurityEvent(item audit.Event) types.SecurityEventItem {
	return types.SecurityEventItem{
		EventType: item.EventType,
		Result:    item.Result,
		UserId:    item.UserID,
		Subject:   item.IdentityValue,
		IP:        item.IP,
		UserAgent: item.UserAgent,
		CreatedAt: item.CreatedAt.Unix(),
	}
}
