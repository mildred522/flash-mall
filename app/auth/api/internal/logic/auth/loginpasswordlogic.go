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

type LoginPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginPasswordLogic {
	return &LoginPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginPasswordLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password required")
	}
	if req.UserId <= 0 && req.Phone == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id or phone required")
	}
	if l.svcCtx.Config.JwtAuthSecret == "" {
		return nil, status.Error(codes.Internal, "jwt secret not configured")
	}

	user, err := l.svcCtx.Store.Authenticate(req.UserId, req.Phone, req.Password)
	if err != nil {
		if err == authstore.ErrInvalidCredentials {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, "authenticate user failed")
	}

	sessionID, refreshToken, err := l.svcCtx.Store.CreateSessionForDevice(user.ID, req.DeviceType, l.svcCtx.Config.RefreshTokenTTLSeconds)
	if err != nil {
		return nil, status.Error(codes.Internal, "create session failed")
	}
	session, ok := l.svcCtx.Store.GetActiveSession(sessionID)
	if !ok {
		return nil, status.Error(codes.Internal, "load session failed")
	}

	resp, err := issueLoginResp(
		l.svcCtx.Config.JwtAuthSecret,
		l.svcCtx.Config.JwtExpireSeconds,
		user.ID,
		session.SessionVersion,
		sessionID,
		refreshToken,
		user.DisplayName,
		user.Phone,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "sign jwt failed")
	}
	return resp, nil
}
