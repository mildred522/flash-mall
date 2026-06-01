package auth

import (
	"context"

	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUserListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserListLogic {
	return &AdminUserListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

type AdminUserItem struct {
	UserId      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
	Role        string `json:"role"`
}

type AdminUserListResp struct {
	Items []AdminUserItem `json:"items"`
	Total int64           `json:"total"`
}

func (l *AdminUserListLogic) AdminUserList() (*AdminUserListResp, error) {
	users := l.svcCtx.Store.ListAllUsers()
	items := make([]AdminUserItem, 0, len(users))
	for _, u := range users {
		items = append(items, AdminUserItem{
			UserId:      u.ID,
			DisplayName: u.DisplayName,
			Phone:       u.Phone,
			Role:        u.Role,
		})
	}
	return &AdminUserListResp{Items: items, Total: int64(len(items))}, nil
}
