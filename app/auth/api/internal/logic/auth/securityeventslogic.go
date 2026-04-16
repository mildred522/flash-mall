package auth

import (
	"context"

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

func mapSecurityEvent(item audit.Event) types.SecurityEventItem {
	return types.SecurityEventItem{
		EventType: item.EventType,
		Result:    item.Result,
		UserId:    item.UserID,
		Subject:   item.IdentityValue,
		IP:        item.IP,
		CreatedAt: item.CreatedAt.Unix(),
	}
}
