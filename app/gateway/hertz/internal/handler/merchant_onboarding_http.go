package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errMerchantOnboardingUnavailable = errors.New("merchant onboarding service unavailable")

func failMerchantOnboarding(ctx context.Context, c *app.RequestContext, operation string, err error) {
	fault, ok := merchantonboarding.AsFault(err)
	if !ok {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
		return
	}
	switch fault.Reason {
	case merchantonboarding.ReasonNotFound:
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, fault.Message))
	case merchantonboarding.ReasonAlreadyActive, merchantonboarding.ReasonAlreadyAudited:
		fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, fault.Message))
	default:
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
	}
}
