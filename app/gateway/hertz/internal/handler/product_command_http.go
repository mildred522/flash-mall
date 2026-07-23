package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/productcommand"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errProductCommandUnavailable = errors.New("product command service unavailable")

func failProductCommand(ctx context.Context, c *app.RequestContext, operation string, err error) {
	fault, ok := productcommand.AsFault(err)
	if !ok {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
		return
	}
	switch fault.Reason {
	case productcommand.ReasonProductNotFound:
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, fault.Message))
	case productcommand.ReasonSupplierNotFound, productcommand.ReasonMerchantNotFound:
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, fault.Message))
	default:
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
	}
}

func productCommandReason(err error) string {
	fault, ok := productcommand.AsFault(err)
	if !ok {
		return ""
	}
	switch fault.Reason {
	case productcommand.ReasonProductNotFound, productcommand.ReasonMerchantNotFound:
		return adminAuditReasonNotFound
	case productcommand.ReasonSupplierNotFound:
		return adminAuditReasonActiveSupplierNotFound
	case productcommand.ReasonInvalidPrice:
		return adminAuditReasonInvalidPrice
	default:
		return ""
	}
}

func productMutationStatusCode(err error) int {
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return consts.StatusBadRequest
	case apperror.CodeProductNotFound, apperror.CodeNotFound:
		return consts.StatusNotFound
	default:
		return consts.StatusBadGateway
	}
}
