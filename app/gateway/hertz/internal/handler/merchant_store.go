package handler

import (
	"context"
	"errors"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/application/merchantstore"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/zeromicro/go-zero/core/logx"
)

var errMerchantStoreServiceUnavailable = errors.New("merchant store service unavailable")

func MerchantStoreProfileHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		if svcCtx.MerchantStores == nil {
			failMerchantStore(ctx, c, "merchant store query failed", errMerchantStoreServiceUnavailable)
			return
		}
		profile, err := svcCtx.MerchantStores.Profile(ctx, merchantID)
		if err != nil {
			failMerchantStore(ctx, c, "merchant store query failed", err)
			return
		}
		ok(ctx, c, profile)
	}
}

func MerchantStoreUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		result := "error"
		var userID, merchantID int64
		defer func() {
			logx.WithContext(ctx).Infof(
				"merchant_store_update result=%s merchant_id=%d user_id=%d request_id=%s duration_ms=%d",
				result, merchantID, userID, tracectx.RequestIDFrom(ctx), time.Since(startedAt).Milliseconds())
		}()
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		userID = identity.UserID
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		var req merchantStoreUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			result = "invalid"
			fail(ctx, c, consts.StatusBadRequest,
				apperror.New(apperror.CodeInvalidArgument, "invalid merchant store request"))
			return
		}
		prepared, err := merchantstore.PrepareUpdate(req)
		if err != nil {
			result = "invalid"
			failMerchantStore(ctx, c, "merchant store update failed", err)
			return
		}
		req = prepared
		merchantID, err = selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		if svcCtx.MerchantStores == nil {
			failMerchantStore(ctx, c, "merchant store update failed", errMerchantStoreServiceUnavailable)
			return
		}
		req.MerchantID = merchantID
		if err := svcCtx.MerchantStores.Update(ctx, req); err != nil {
			if fault, ok := merchantstore.AsFault(err); ok {
				if fault.Reason == merchantstore.ReasonInvalidArgument {
					result = "invalid"
				} else if fault.Reason == merchantstore.ReasonVersionConflict {
					result = "conflict"
				}
			}
			failMerchantStore(ctx, c, "merchant store update failed", err)
			return
		}
		profile, err := svcCtx.MerchantStores.Profile(ctx, merchantID)
		if err != nil {
			failMerchantStore(ctx, c, "merchant store query failed", err)
			return
		}
		result = "success"
		invalidateStoreReadCaches(ctx, svcCtx, merchantID)
		ok(ctx, c, profile)
	}
}

func failMerchantStore(ctx context.Context, c *app.RequestContext, operation string, err error) {
	if fault, ok := merchantstore.AsFault(err); ok {
		switch fault.Reason {
		case merchantstore.ReasonNotFound:
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeMerchantNotFound, fault.Message))
		case merchantstore.ReasonVersionConflict:
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, fault.Message))
		default:
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
		}
		return
	}
	fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
}
