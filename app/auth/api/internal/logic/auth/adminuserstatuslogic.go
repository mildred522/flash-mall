package auth

import (
	"context"
	"errors"
	"fmt"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminUserStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type AdminUserStatusResp struct {
	UserId     int64  `json:"user_id"`
	Status     int64  `json:"status"`
	StatusText string `json:"status_text"`
}

func NewAdminUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserStatusLogic {
	return &AdminUserStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUserStatusLogic) AdminUserStatus(userID int64, newStatus int64) (*AdminUserStatusResp, error) {
	if userID <= 0 {
		l.RecordFailure(userID, newStatus, adminAuditReasonInvalidUserID)
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	user, err := l.svcCtx.Store.SetUserStatus(userID, newStatus)
	if err != nil {
		switch {
		case errors.Is(err, authstore.ErrUserNotFound):
			l.RecordFailure(userID, newStatus, adminAuditReasonNotFound)
			return nil, status.Error(codes.NotFound, "user not found")
		case errors.Is(err, authstore.ErrInvalidCredentials):
			l.RecordFailure(userID, newStatus, adminAuditReasonInvalidStatus)
			return nil, status.Error(codes.InvalidArgument, "status must be 1 or 2")
		default:
			l.RecordFailure(userID, newStatus, adminAuditReasonStoreFailed)
			return nil, status.Error(codes.Internal, "update user status failed")
		}
	}
	if adminID, ok := parseUserIDClaim(l.ctx.Value("user_id")); ok {
		recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
			EventType:     adminUserStatusAuditEvent(user.Status),
			Result:        adminAuditResultSuccess,
			UserID:        adminID,
			IdentityValue: fmt.Sprintf("target_user:%d phone:%s status:%s", user.ID, user.Phone, adminUserStatusText(user.Status)),
		})
	}
	return &AdminUserStatusResp{
		UserId:     user.ID,
		Status:     adminUserStatus(user.Status),
		StatusText: adminUserStatusText(user.Status),
	}, nil
}

func (l *AdminUserStatusLogic) RecordFailure(userID int64, newStatus int64, reason string) {
	if adminID, ok := parseUserIDClaim(l.ctx.Value("user_id")); ok {
		recordAuditEvent(l.ctx, l.svcCtx, l.Logger, audit.Event{
			EventType:     adminUserStatusAuditEvent(newStatus),
			Result:        adminAuditResultFail,
			UserID:        adminID,
			IdentityValue: fmt.Sprintf("target_user:%d status:%s reason:%s", userID, adminUserStatusText(newStatus), reason),
		})
	}
}

func adminUserStatusAuditEvent(status int64) string {
	switch adminUserStatus(status) {
	case 1:
		return adminAuditUserEnabled
	case 2:
		return adminAuditUserDisabled
	default:
		return adminAuditUserStatusUpdateFailed
	}
}
