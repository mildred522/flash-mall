package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/svc"
	"flash-mall/app/auth/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeLogic {
	return &MeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MeLogic) Me() (*types.MeResp, error) {
	userID, ok := parseUserIDClaim(l.ctx.Value("user_id"))
	if !ok {
		userID, ok = parseUserIDClaim(l.ctx.Value("sub"))
	}
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user subject in jwt")
	}

	sessionID, _ := l.ctx.Value("sid").(string)
	if sessionID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing session id in jwt")
	}
	session, err := l.svcCtx.Store.GetActiveSession(sessionID)
	if errors.Is(err, authstore.ErrSessionNotFound) {
		return nil, status.Error(codes.Unauthenticated, "session invalid or expired")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "load session failed")
	}
	if session == nil || session.UserID != userID {
		return nil, status.Error(codes.Unauthenticated, "session invalid or expired")
	}
	sessionVersion, ok := parseUserIDClaim(l.ctx.Value("session_version"))
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing session version in jwt")
	}
	if session.SessionVersion != sessionVersion {
		return nil, status.Error(codes.Unauthenticated, "session version mismatch")
	}

	user, err := l.svcCtx.Store.GetUserByID(userID)
	if errors.Is(err, authstore.ErrUserNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "load user failed")
	}

	return &types.MeResp{
		UserId:      userID,
		DisplayName: user.DisplayName,
		Phone:       user.Phone,
		Role:        user.Role,
	}, nil
}

func parseUserIDClaim(v any) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int32:
		return int64(val), true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	case json.Number:
		id, err := val.Int64()
		if err != nil {
			return 0, false
		}
		return id, true
	case string:
		id, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, false
		}
		return id, true
	default:
		return 0, false
	}
}
