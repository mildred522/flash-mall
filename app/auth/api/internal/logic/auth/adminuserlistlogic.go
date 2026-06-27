package auth

import (
	"context"

	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	Status      int64  `json:"status"`
	StatusText  string `json:"status_text"`
	CreateTime  string `json:"create_time"`
}

type AdminUserListResp struct {
	Items []AdminUserItem `json:"items"`
	Total int64           `json:"total"`
}

func (l *AdminUserListLogic) AdminUserList(page, pageSize, statusFilter int64, role, keyword string) (*AdminUserListResp, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if statusFilter != 1 && statusFilter != 2 {
		statusFilter = 0
	}
	users, total, err := l.svcCtx.Store.ListUsers(page, pageSize, statusFilter, role, keyword)
	if err != nil {
		l.Errorf("admin user list failed: %v", err)
		return nil, status.Error(codes.Internal, "list users failed")
	}
	items := make([]AdminUserItem, 0, len(users))
	for _, u := range users {
		items = append(items, adminUserItemFromStoreUser(u))
	}
	return &AdminUserListResp{Items: items, Total: total}, nil
}

func adminUserItemFromStoreUser(u *authstore.User) AdminUserItem {
	if u == nil {
		return AdminUserItem{}
	}
	return AdminUserItem{
		UserId:      u.ID,
		DisplayName: u.DisplayName,
		Phone:       u.Phone,
		Role:        u.Role,
		Status:      adminUserStatus(u.Status),
		StatusText:  adminUserStatusText(u.Status),
		CreateTime:  u.CreateTime,
	}
}

func adminUserStatus(status int64) int64 {
	if status == 0 {
		return 1
	}
	return status
}

func adminUserStatusText(status int64) string {
	switch adminUserStatus(status) {
	case 1:
		return "active"
	case 2:
		return "disabled"
	default:
		return "unknown"
	}
}
