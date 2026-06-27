package auth

import (
	"context"
	"errors"

	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminUserDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserDetailLogic {
	return &AdminUserDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUserDetailLogic) AdminUserDetail(userID int64) (*AdminUserItem, error) {
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	user, err := l.svcCtx.Store.GetUserByIDAnyStatus(userID)
	if errors.Is(err, authstore.ErrUserNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		l.Errorf("admin user detail failed: %v", err)
		return nil, status.Error(codes.Internal, "get user detail failed")
	}
	item := adminUserItemFromStoreUser(user)
	return &item, nil
}
