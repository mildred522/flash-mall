package logic

import (
	"flash-mall/app/product/rpc/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func requireLegacyStockMutation(svcCtx *svc.ServiceContext) error {
	if svcCtx != nil && svcCtx.Config.LegacyStockMutationEnabled {
		return nil
	}
	return status.Error(codes.FailedPrecondition, "legacy product stock mutation is disabled; use inventory-kitex")
}
