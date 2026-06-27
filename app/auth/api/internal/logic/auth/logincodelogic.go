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
	if err := l.svcCtx.Store.ConsumeCode(req.Phone, "login", req.Code, l.svcCtx.Config.VerificationCodeMaxAttempts); err != nil {
		if errors.Is(err, authstore.ErrInvalidCode) {
			recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
				EventType:     auditEventLoginCodeFail,
				Result:        auditResultFail,
				UserID:        auditUserIDByPhone(l.svcCtx, req.Phone),
				IdentityValue: req.Phone,
				IP:            req.ClientIP,
				UserAgent:     req.UserAgent,
			})
			return nil, status.Error(codes.InvalidArgument, "invalid verification code")
		}
		return nil, status.Error(codes.Internal, "verify code failed")
	}
	user, err := l.svcCtx.Store.GetUserByPhone(req.Phone)
	if errors.Is(err, authstore.ErrUserNotFound) {
		recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
			EventType:     auditEventLoginCodeFail,
			Result:        auditResultFail,
			IdentityValue: req.Phone,
			IP:            req.ClientIP,
			UserAgent:     req.UserAgent,
		})
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "load user failed")
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
		EventType:     auditEventLoginCodeSuccess,
		Result:        auditResultSuccess,
		UserID:        user.ID,
		IdentityValue: user.Phone,
		IP:            req.ClientIP,
		UserAgent:     req.UserAgent,
	})
	return resp, nil
}
