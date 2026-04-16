package auth

import (
	"context"

	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshLogic) Refresh(refreshToken string) (*types.LoginResp, error) {
	if refreshToken == "" {
		return nil, status.Error(codes.Unauthenticated, "missing refresh token")
	}

	session, newRefreshToken, err := l.svcCtx.Store.RefreshSession(refreshToken, l.svcCtx.Config.RefreshTokenTTLSeconds)
	if err != nil {
		if err == authstore.ErrRefreshTokenInvalid {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		if err == authstore.ErrRefreshTokenReplayed {
			return nil, status.Error(codes.Unauthenticated, "refresh token replayed")
		}
		return nil, status.Error(codes.Internal, "refresh session failed")
	}

	user, ok := l.svcCtx.Store.GetUserByID(session.UserID)
	if !ok {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	resp, err := issueLoginResp(
		l.svcCtx.Config.JwtAuthSecret,
		l.svcCtx.Config.JwtExpireSeconds,
		user.ID,
		session.SessionVersion,
		session.ID,
		newRefreshToken,
		user.DisplayName,
		user.Phone,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "sign jwt failed")
	}
	return resp, nil
}
