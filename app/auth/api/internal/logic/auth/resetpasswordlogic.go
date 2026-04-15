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

type ResetPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordLogic {
	return &ResetPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetPasswordLogic) Reset(req *types.ResetPasswordReq) error {
	if req.Phone == "" || req.Code == "" || req.NewPassword == "" {
		return status.Error(codes.InvalidArgument, "phone, code and new_password are required")
	}
	if err := l.svcCtx.Store.ConsumeCode(req.Phone, "reset-password", req.Code); err != nil {
		if err == authstore.ErrInvalidCode {
			return status.Error(codes.InvalidArgument, "invalid verification code")
		}
		return status.Error(codes.Internal, "verify code failed")
	}
	if _, err := l.svcCtx.Store.UpdatePassword(req.Phone, req.NewPassword); err != nil {
		if err == authstore.ErrUserNotFound {
			return status.Error(codes.NotFound, "user not found")
		}
		return status.Error(codes.Internal, "update password failed")
	}
	return nil
}
