package auth

import (
	"context"

	"flash-mall/app/auth/api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogoutAllLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutAllLogic {
	return &LogoutAllLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutAllLogic) LogoutAll() error {
	userID, ok := parseUserIDClaim(l.ctx.Value("user_id"))
	if !ok {
		return status.Error(codes.Unauthenticated, "missing user_id in jwt")
	}
	l.svcCtx.Store.LogoutAll(userID)
	return nil
}
