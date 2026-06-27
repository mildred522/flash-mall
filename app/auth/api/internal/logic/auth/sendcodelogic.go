package auth

import (
	"context"
	"fmt"
	"time"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SendCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendCodeLogic {
	return &SendCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendCodeLogic) Send(req *types.SendCodeReq) (*types.SendCodeResp, error) {
	if req.Phone == "" {
		return nil, status.Error(codes.InvalidArgument, "phone required")
	}
	if req.Scene == "" {
		return nil, status.Error(codes.InvalidArgument, "scene required")
	}
	if err := l.checkSendCodeRisk(req); err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
				EventType:     auditEventSendCodeBlocked,
				Result:        auditResultBlocked,
				UserID:        auditUserIDByPhone(l.svcCtx, req.Phone),
				IdentityValue: req.Phone,
				IP:            req.ClientIP,
				UserAgent:     req.UserAgent,
			})
		}
		return nil, err
	}
	code, expiresAt, err := l.svcCtx.Store.IssueCode(req.Phone, req.Scene, l.svcCtx.Config.CodeTTLSeconds)
	if err != nil {
		return nil, status.Error(codes.Internal, "issue verification code failed")
	}
	if err := l.recordSendCodeRisk(req); err != nil {
		l.Errorf("record code send risk failed after issue: phone=%s scene=%s err=%v", req.Phone, req.Scene, err)
	}
	recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
		EventType:     auditEventSendCodeSuccess,
		Result:        auditResultSuccess,
		UserID:        auditUserIDByPhone(l.svcCtx, req.Phone),
		IdentityValue: req.Phone,
		IP:            req.ClientIP,
		UserAgent:     req.UserAgent,
	})

	return &types.SendCodeResp{
		Sent:      true,
		ExpiresAt: expiresAt.Unix(),
		DebugCode: code,
	}, nil
}

func (l *SendCodeLogic) checkSendCodeRisk(req *types.SendCodeReq) error {
	limiter := l.svcCtx.RiskLimiter
	if limiter == nil {
		return nil
	}
	if blocked, _, err := limiter.Blocked(l.ctx, sendCodeCooldownKey(req.Phone, req.Scene), 1); err != nil {
		return status.Error(codes.Internal, "check code send risk failed")
	} else if blocked {
		return status.Error(codes.ResourceExhausted, "verification code request too frequent")
	}
	if blocked, _, err := limiter.Blocked(l.ctx, sendCodePhoneWindowKey(req.Phone, req.Scene), l.svcCtx.Config.CodeSendPhoneMaxAttempts); err != nil {
		return status.Error(codes.Internal, "check code send risk failed")
	} else if blocked {
		return status.Error(codes.ResourceExhausted, "too many verification code requests for this phone")
	}
	if req.ClientIP != "" {
		if blocked, _, err := limiter.Blocked(l.ctx, sendCodeIPWindowKey(req.ClientIP, req.Scene), l.svcCtx.Config.CodeSendIPMaxAttempts); err != nil {
			return status.Error(codes.Internal, "check code send risk failed")
		} else if blocked {
			return status.Error(codes.ResourceExhausted, "too many verification code requests from this IP")
		}
	}
	return nil
}

func (l *SendCodeLogic) recordSendCodeRisk(req *types.SendCodeReq) error {
	limiter := l.svcCtx.RiskLimiter
	if limiter == nil {
		return nil
	}
	if err := limiter.Incr(l.ctx, sendCodeCooldownKey(req.Phone, req.Scene), sendCodeCooldownTTL(l.svcCtx.Config.CodeSendCooldownSeconds)); err != nil {
		return status.Error(codes.Internal, "record code send risk failed")
	}
	if err := limiter.Incr(l.ctx, sendCodePhoneWindowKey(req.Phone, req.Scene), sendCodeWindowTTL(l.svcCtx.Config.CodeSendPhoneWindowSeconds)); err != nil {
		return status.Error(codes.Internal, "record code send risk failed")
	}
	if req.ClientIP != "" {
		if err := limiter.Incr(l.ctx, sendCodeIPWindowKey(req.ClientIP, req.Scene), sendCodeWindowTTL(l.svcCtx.Config.CodeSendIPWindowSeconds)); err != nil {
			return status.Error(codes.Internal, "record code send risk failed")
		}
	}
	return nil
}

func sendCodeCooldownKey(phone, scene string) string {
	return fmt.Sprintf("auth:risk:code:phone:%s:%s:cooldown", scene, phone)
}

func sendCodePhoneWindowKey(phone, scene string) string {
	return fmt.Sprintf("auth:risk:code:phone:%s:%s:window", scene, phone)
}

func sendCodeIPWindowKey(clientIP, scene string) string {
	return fmt.Sprintf("auth:risk:code:ip:%s:%s", scene, clientIP)
}

func sendCodeCooldownTTL(seconds int64) time.Duration {
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func sendCodeWindowTTL(seconds int64) time.Duration {
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}
