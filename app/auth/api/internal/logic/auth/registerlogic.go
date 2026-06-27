package auth

import (
	"context"
	"errors"

	"flash-mall/app/auth/api/internal/audit"
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
	if err := l.svcCtx.Store.ConsumeCode(req.Phone, "register", req.Code, l.svcCtx.Config.VerificationCodeMaxAttempts); err != nil {
		if errors.Is(err, authstore.ErrInvalidCode) {
			recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
				EventType:     auditEventRegisterFail,
				Result:        auditResultFail,
				IdentityValue: req.Phone,
				IP:            req.ClientIP,
				UserAgent:     req.UserAgent,
			})
			return nil, status.Error(codes.InvalidArgument, "invalid verification code")
		}
		return nil, status.Error(codes.Internal, "verify code failed")
	}

	user, err := l.svcCtx.Store.CreateUser(req.Phone, req.DisplayName, req.Password)
	if err != nil {
		if errors.Is(err, authstore.ErrUserExists) {
			recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
				EventType:     auditEventRegisterFail,
				Result:        auditResultFail,
				UserID:        auditUserIDByPhone(l.svcCtx, req.Phone),
				IdentityValue: req.Phone,
				IP:            req.ClientIP,
				UserAgent:     req.UserAgent,
			})
			return nil, status.Error(codes.AlreadyExists, "phone already registered")
		}
		return nil, status.Error(codes.Internal, "create user failed")
	}

	sessionID, refreshToken, err := l.svcCtx.Store.CreateSessionForDevice(user.ID, req.DeviceType, l.svcCtx.Config.RefreshTokenTTLSeconds)
	if err != nil {
		return nil, status.Error(codes.Internal, "create session failed")
	}
	session, err := l.svcCtx.Store.GetActiveSession(sessionID)
	if err != nil {
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
		user.Role,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "sign jwt failed")
	}
	recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
		EventType:     auditEventRegisterSuccess,
		Result:        auditResultSuccess,
		UserID:        user.ID,
		IdentityValue: user.Phone,
		IP:            req.ClientIP,
		UserAgent:     req.UserAgent,
	})
	return resp, nil
}
