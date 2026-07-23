package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/application/useraddress"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errUserAddressUnavailable = errors.New("user address service unavailable")

func UserAddressListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		if svcCtx.UserAddresses == nil {
			failUserAddress(ctx, c, "user address query failed", errUserAddressUnavailable)
			return
		}
		result, err := svcCtx.UserAddresses.List(ctx, identity.UserID)
		if err != nil {
			failUserAddress(ctx, c, "user address query failed", err)
			return
		}
		ok(ctx, c, result)
	}
}

func UserAddressUpsertHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		var req UserAddressUpsertReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest,
				apperror.New(apperror.CodeInvalidArgument, "invalid user address request"))
			return
		}
		if svcCtx.UserAddresses == nil {
			failUserAddress(ctx, c, "user address save failed", errUserAddressUnavailable)
			return
		}
		req.UserID = identity.UserID
		result, err := svcCtx.UserAddresses.Upsert(ctx, req)
		if err != nil {
			failUserAddress(ctx, c, "user address save failed", err)
			return
		}
		ok(ctx, c, result)
	}
}

func failUserAddress(ctx context.Context, c *app.RequestContext, operation string, err error) {
	if fault, ok := useraddress.AsFault(err); ok {
		switch fault.Reason {
		case useraddress.ReasonNotFound:
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, fault.Message))
		default:
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
		}
		return
	}
	fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
}
