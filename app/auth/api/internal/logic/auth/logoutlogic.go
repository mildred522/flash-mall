package auth

import (
	"context"

	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(refreshToken string) error {
	if refreshToken == "" {
		return status.Error(codes.Unauthenticated, "missing refresh token")
	}
	if err := l.svcCtx.Store.Logout(refreshToken); err != nil {
		if err == authstore.ErrRefreshTokenInvalid {
			return status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		return status.Error(codes.Internal, "logout failed")
	}
	return nil
}
