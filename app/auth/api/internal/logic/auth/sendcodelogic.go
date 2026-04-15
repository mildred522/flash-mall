package auth

import (
	"context"

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

	code, expiresAt, err := l.svcCtx.Store.IssueCode(req.Phone, req.Scene, l.svcCtx.Config.CodeTTLSeconds)
	if err != nil {
		return nil, status.Error(codes.Internal, "issue verification code failed")
	}

	return &types.SendCodeResp{
		Sent:      true,
		ExpiresAt: expiresAt.Unix(),
		DebugCode: code,
	}, nil
}
