package auth

import (
	"context"
	"errors"

	"flash-mall/app/auth/api/internal/audit"
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
		if errors.Is(err, authstore.ErrRefreshTokenInvalid) {
			return status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		if errors.Is(err, authstore.ErrRefreshTokenReplayed) {
			return status.Error(codes.Unauthenticated, "refresh token replayed")
		}
		return status.Error(codes.Internal, "logout failed")
	}
	if userID, ok := parseUserIDClaim(l.ctx.Value("user_id")); ok {
		recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
			EventType:     auditEventLogoutSuccess,
			Result:        auditResultSuccess,
			UserID:        userID,
			IdentityValue: auditIdentity("", userID),
		})
	}
	return nil
}
