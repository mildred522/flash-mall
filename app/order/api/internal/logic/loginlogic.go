package logic

import (
	"context"
	"time"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	if req.Password == "" || req.Password != l.svcCtx.Config.AuthDemoPassword {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	secret := l.svcCtx.Config.JwtAuthSecret
	if secret == "" {
		return nil, status.Error(codes.Internal, "jwt secret not configured")
	}

	expireSeconds := l.svcCtx.Config.JwtExpireSeconds
	if expireSeconds <= 0 {
		expireSeconds = 2 * 60 * 60
	}
	now := time.Now()
	expireAt := now.Add(time.Duration(expireSeconds) * time.Second)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": req.UserId,
		"role":    "user",
		"iat":     now.Unix(),
		"exp":     expireAt.Unix(),
	})
	accessToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, status.Error(codes.Internal, "sign jwt failed")
	}

	return &types.LoginResp{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresAt:   expireAt.Unix(),
	}, nil
}
