package auth

import (
	"context"
	"errors"
	"strconv"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func recordAuditEvent(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, event audit.Event) {
	if svcCtx == nil || svcCtx.AuditRecorder == nil {
		return
	}
	if _, ok := svcCtx.AuditRecorder.(*audit.RedisRecorder); ok {
		go func() {
			if err := svcCtx.AuditRecorder.Record(context.Background(), event); err != nil {
				logger.Errorf("record audit event failed: type=%s result=%s user_id=%d err=%v", event.EventType, event.Result, event.UserID, err)
			}
		}()
		return
	}
	if err := svcCtx.AuditRecorder.Record(ctx, event); err != nil {
		logger.Errorf("record audit event failed: type=%s result=%s user_id=%d err=%v", event.EventType, event.Result, event.UserID, err)
	}
}

func auditIdentity(phone string, userID int64) string {
	if phone != "" {
		return phone
	}
	if userID > 0 {
		return strconv.FormatInt(userID, 10)
	}
	return ""
}

func auditUserIDByPhone(svcCtx *svc.ServiceContext, phone string) int64 {
	if svcCtx == nil || svcCtx.Store == nil || phone == "" {
		return 0
	}
	user, err := svcCtx.Store.GetUserByPhone(phone)
	if err != nil {
		if !errors.Is(err, authstore.ErrUserNotFound) {
			logx.WithContext(context.Background()).Errorf("audit user lookup failed: %v", err)
		}
		return 0
	}
	return user.ID
}
