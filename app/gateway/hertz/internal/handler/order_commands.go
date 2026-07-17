package handler

import (
	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
)

func requireOrderCommands(svcCtx *svc.ServiceContext) (ports.OrderCommands, error) {
	if svcCtx.OrderCommands == nil {
		return nil, apperror.New(apperror.CodeInternal, "order command adapter is not configured")
	}
	return svcCtx.OrderCommands, nil
}
