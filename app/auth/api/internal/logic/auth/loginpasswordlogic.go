package auth

import (
	"context"
	"fmt"
	"time"

	"flash-mall/app/auth/api/internal/audit"
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
	if err := l.checkLoginRisk(req); err != nil {
		return nil, err
	}

	user, err := l.svcCtx.Store.Authenticate(req.UserId, req.Phone, req.Password)
	if err != nil {
		if err == authstore.ErrInvalidCredentials {
			if incrErr := l.recordLoginFailure(req); incrErr != nil {
				return nil, incrErr
			}
			recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
				EventType:     "login_password_fail",
				Result:        "fail",
				UserID:        auditUserIDByPhone(l.svcCtx, req.Phone),
				IdentityValue: auditIdentity(req.Phone, req.UserId),
				IP:            req.ClientIP,
				UserAgent:     req.UserAgent,
			})
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
		user.Role,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "sign jwt failed")
	}
	l.resetLoginRisk(req)
	recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
		EventType:     "login_password_success",
		Result:        "success",
		UserID:        user.ID,
		IdentityValue: user.Phone,
		IP:            req.ClientIP,
		UserAgent:     req.UserAgent,
	})
	return resp, nil
}

func (l *LoginPasswordLogic) checkLoginRisk(req *types.LoginReq) error {
	limiter := l.svcCtx.RiskLimiter
	if limiter == nil {
		return nil
	}
	if accountKey := loginAccountRiskKey(req); accountKey != "" {
		blocked, _, err := limiter.Blocked(l.ctx, accountKey, l.svcCtx.Config.LoginFailPhoneMaxAttempts)
		if err != nil {
			return status.Error(codes.Internal, "check login risk failed")
		}
		if blocked {
			return status.Error(codes.ResourceExhausted, "too many login attempts, please try again later")
		}
	}
	if req.ClientIP != "" {
		blocked, _, err := limiter.Blocked(l.ctx, loginIPRiskKey(req.ClientIP), l.svcCtx.Config.LoginFailIPMaxAttempts)
		if err != nil {
			return status.Error(codes.Internal, "check login risk failed")
		}
		if blocked {
			return status.Error(codes.ResourceExhausted, "too many login attempts, please try again later")
		}
	}
	return nil
}

func (l *LoginPasswordLogic) recordLoginFailure(req *types.LoginReq) error {
	limiter := l.svcCtx.RiskLimiter
	if limiter == nil {
		return nil
	}
	if accountKey := loginAccountRiskKey(req); accountKey != "" {
		if err := limiter.Incr(l.ctx, accountKey, loginFailWindow(l.svcCtx.Config.LoginFailWindowSeconds)); err != nil {
			return status.Error(codes.Internal, "record login risk failed")
		}
	}
	if req.ClientIP != "" {
		if err := limiter.Incr(l.ctx, loginIPRiskKey(req.ClientIP), loginFailWindow(l.svcCtx.Config.LoginFailWindowSeconds)); err != nil {
			return status.Error(codes.Internal, "record login risk failed")
		}
	}
	return nil
}

func (l *LoginPasswordLogic) resetLoginRisk(req *types.LoginReq) {
	limiter := l.svcCtx.RiskLimiter
	if limiter == nil {
		return
	}
	if accountKey := loginAccountRiskKey(req); accountKey != "" {
		_ = limiter.Reset(l.ctx, accountKey)
	}
}

func loginPhoneRiskKey(phone string) string {
	return fmt.Sprintf("auth:risk:login:phone:%s", phone)
}

func loginUserRiskKey(userID int64) string {
	return fmt.Sprintf("auth:risk:login:user:%d", userID)
}

func loginIPRiskKey(clientIP string) string {
	return fmt.Sprintf("auth:risk:login:ip:%s", clientIP)
}

func loginAccountRiskKey(req *types.LoginReq) string {
	if req == nil {
		return ""
	}
	if req.Phone != "" {
		return loginPhoneRiskKey(req.Phone)
	}
	if req.UserId > 0 {
		return loginUserRiskKey(req.UserId)
	}
	return ""
}

func loginFailWindow(seconds int64) time.Duration {
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}
