package handler

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminReconciliationListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adminReconciliationQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		if svcCtx.Reconciliation == nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.New(apperror.CodeInternal, "reconciliation service unavailable"))
			return
		}
		resp, err := svcCtx.Reconciliation.List(ctx, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.Wrap(apperror.CodeInternal, "reconciliation query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func AdminReconciliationScanHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if svcCtx.Reconciliation == nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.New(apperror.CodeInternal, "reconciliation service unavailable"))
			return
		}
		inserted, err := svcCtx.Reconciliation.Scan(ctx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.Wrap(apperror.CodeInternal, "reconciliation scan failed", err))
			return
		}
		ok(ctx, c, map[string]any{"inserted": inserted})
	}
}

func adminReconciliationQueryFromRequest(c *app.RequestContext) (AdminReconciliationReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminReconciliationReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminReconciliationReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminReconciliationReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return AdminReconciliationReq{Page: page, PageSize: pageSize, Status: status,
		IssueType: c.Query("issue_type"), OrderID: c.Query("order_id")}, nil
}
