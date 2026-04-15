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

type LoginCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginCodeLogic {
	return &LoginCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginCodeLogic) Login(req *types.LoginCodeReq) (*types.LoginResp, error) {
	if req.Phone == "" || req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "phone and code are required")
	}
	if err := l.svcCtx.Store.ConsumeCode(req.Phone, "login", req.Code); err != nil {
		if err == authstore.ErrInvalidCode {
			return nil, status.Error(codes.InvalidArgument, "invalid verification code")
		}
		return nil, status.Error(codes.Internal, "verify code failed")
	}
	user, ok := l.svcCtx.Store.GetUserByPhone(req.Phone)
	if !ok {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	sessionID, refreshToken, err := l.svcCtx.Store.CreateSessionForDevice(user.ID, req.DeviceType, l.svcCtx.Config.RefreshTokenTTLSeconds)
	if err != nil {
		return nil, status.Error(codes.Internal, "create session failed")
	}
	session, sessionOK := l.svcCtx.Store.GetActiveSession(sessionID)
	if !sessionOK {
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
