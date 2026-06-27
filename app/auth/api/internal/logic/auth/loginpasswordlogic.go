package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/risk"
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
		if errors.Is(err, authstore.ErrInvalidCredentials) {
			if incrErr := l.recordLoginFailure(req); incrErr != nil {
				return nil, incrErr
			}
			recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
				EventType:     auditEventLoginPasswordFail,
				Result:        auditResultFail,
				UserID:        auditUserIDByPhone(l.svcCtx, req.Phone),
				IdentityValue: auditIdentity(req.Phone, req.UserId),
				IP:            req.ClientIP,
				UserAgent:     req.UserAgent,
			})
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		l.Errorf("authenticate user failed: phone=%s user_id=%d err=%v", req.Phone, req.UserId, err)
		return nil, status.Error(codes.Internal, "authenticate user failed")
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
	l.resetLoginRisk(req)
	recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
		EventType:     auditEventLoginPasswordSuccess,
		Result:        auditResultSuccess,
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
	type riskCheck struct {
		key string
		max int64
	}
	checks := make([]riskCheck, 0, 2)
	if accountKey := loginAccountRiskKey(req); accountKey != "" {
		checks = append(checks, riskCheck{key: accountKey, max: l.svcCtx.Config.LoginFailPhoneMaxAttempts})
	}
	if req.ClientIP != "" {
		checks = append(checks, riskCheck{key: loginIPRiskKey(req.ClientIP), max: l.svcCtx.Config.LoginFailIPMaxAttempts})
	}
	if len(checks) == 0 {
		return nil
	}

	errCh := make(chan error, len(checks))
	for _, check := range checks {
		check := check
		go func() {
			blocked, _, err := limiter.Blocked(l.ctx, check.key, check.max)
			if err != nil {
				errCh <- status.Error(codes.Internal, "check login risk failed")
				return
			}
			if blocked {
				errCh <- status.Error(codes.ResourceExhausted, "too many login attempts, please try again later")
				return
			}
			errCh <- nil
		}()
	}
	for range checks {
		if err := <-errCh; err != nil {
			return err
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
		if _, ok := limiter.(*risk.RedisLimiter); ok {
			go func() {
				_ = limiter.Reset(context.Background(), accountKey)
			}()
			return
		}
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
