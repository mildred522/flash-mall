package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/supplier"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errSupplierServiceUnavailable = errors.New("supplier service unavailable")

func AdminSupplierListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, appErr := parseSupplierListQuery(c)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		if svcCtx.Suppliers == nil {
			failSupplier(ctx, c, "admin supplier query failed", errSupplierServiceUnavailable)
			return
		}
		items, total, err := svcCtx.Suppliers.List(ctx, req)
		if err != nil {
			failSupplier(ctx, c, "admin supplier query failed", err)
			return
		}
		ok(ctx, c, AdminSupplierListResp{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
	}
}

func AdminSupplierDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		supplierID, err := parsePositiveInt64(c.Query("supplier_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "supplier_id required"))
			return
		}

		if svcCtx.Suppliers == nil {
			failSupplier(ctx, c, "admin supplier detail query failed", errSupplierServiceUnavailable)
			return
		}
		item, err := svcCtx.Suppliers.Detail(ctx, supplierID)
		if err != nil {
			failSupplier(ctx, c, "admin supplier detail query failed", err)
			return
		}
		ok(ctx, c, item)
	}
}

func AdminSupplierCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminSupplierCreateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid supplier create request"))
			return
		}
		if svcCtx.Suppliers == nil {
			failSupplier(ctx, c, "admin supplier create failed", errSupplierServiceUnavailable)
			return
		}
		result, err := svcCtx.Suppliers.Create(ctx, req)
		if err != nil {
			failSupplier(ctx, c, "admin supplier create failed", err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditSupplierCreated,
			fmt.Sprintf("supplier:%d name:%s", result.SupplierID, strings.TrimSpace(req.Name)))
		ok(ctx, c, AdminSupplierCreateResp{SupplierID: result.SupplierID})
	}
}

func AdminSupplierUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminSupplierUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid supplier update request"))
			return
		}
		if svcCtx.Suppliers == nil {
			failSupplier(ctx, c, "admin supplier update failed", errSupplierServiceUnavailable)
			return
		}
		if err := svcCtx.Suppliers.Update(ctx, req); err != nil {
			recordSupplierMutationFailure(c, svcCtx, req, err)
			failSupplier(ctx, c, "admin supplier update failed", err)
			return
		}
		recordGatewayAdminAuditEvent(c, svcCtx, supplierUpdateAuditEvent(req.Status), fmt.Sprintf("supplier:%d", req.SupplierID))
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func parseSupplierListQuery(c *app.RequestContext) (supplier.ListQuery, *apperror.Error) {
	page, err := parseInt64Default(c.Query("page"), defaultProductPage)
	if err != nil || page <= 0 {
		return supplier.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page must be positive")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), defaultProductPageSize)
	if err != nil || pageSize <= 0 {
		return supplier.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive")
	}
	if pageSize > maxProductPageSize {
		pageSize = maxProductPageSize
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return supplier.ListQuery{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return supplier.ListQuery{
		Page:     page,
		PageSize: pageSize,
		Status:   status,
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	}, nil
}

func failSupplier(ctx context.Context, c *app.RequestContext, operation string, err error) {
	fault, ok := supplier.AsFault(err)
	if !ok {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
		return
	}
	switch fault.Reason {
	case supplier.ReasonSupplierNotFound:
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, fault.Message))
	case supplier.ReasonHasActiveProducts:
		fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, fault.Message))
	default:
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
	}
}

func recordSupplierMutationFailure(c *app.RequestContext, svcCtx *svc.ServiceContext, request supplier.UpdateInput, err error) {
	fault, ok := supplier.AsFault(err)
	if !ok {
		return
	}
	reason := ""
	if fault.Reason == supplier.ReasonSupplierNotFound {
		reason = adminAuditReasonNotFound
	} else if fault.Reason == supplier.ReasonHasActiveProducts {
		reason = adminAuditReasonHasActiveProducts
	}
	if reason != "" {
		recordGatewayAdminAuditFailure(c, svcCtx, supplierUpdateAuditEvent(request.Status),
			fmt.Sprintf("supplier:%d reason:%s", request.SupplierID, reason))
	}
}
