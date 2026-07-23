package handler

import (
	"context"
	"fmt"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func writeStockAdjustError(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, req AdminProductStockAdjustReq, err error) {
	switch apperror.CodeOf(err) {
	case apperror.CodeStockNotFound, apperror.CodeProductNotFound, apperror.CodeNotFound:
		recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonNotFound))
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
	case apperror.CodeStockInsufficient:
		recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d delta:%d bucket:%d reason:%s", req.ProductID, req.Delta, req.BucketIdx, adminAuditReasonInsufficientOrMissingStock))
		fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeStockInsufficient, "stock bucket not found or insufficient stock"))
	default:
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product stock adjust failed", err))
	}
}
