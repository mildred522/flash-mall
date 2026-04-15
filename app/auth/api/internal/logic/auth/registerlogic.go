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

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterReq) (*types.LoginResp, error) {
	if req.Phone == "" || req.Password == "" || req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "phone, password and code are required")
	}
	if err := l.svcCtx.Store.ConsumeCode(req.Phone, "register", req.Code); err != nil {
		if err == authstore.ErrInvalidCode {
			return nil, status.Error(codes.InvalidArgument, "invalid verification code")
		}
		return nil, status.Error(codes.Internal, "verify code failed")
	}

	user, err := l.svcCtx.Store.CreateUser(req.Phone, req.DisplayName, req.Password)
	if err != nil {
		if err == authstore.ErrUserExists {
			return nil, status.Error(codes.AlreadyExists, "phone already registered")
		}
		return nil, status.Error(codes.Internal, "create user failed")
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
