package handler

import (
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/orderquery"
	"flash-mall/app/gateway/hertz/internal/svc"
)

func orderQueryService(svcCtx *svc.ServiceContext) (*orderquery.Service, error) {
	if svcCtx.OrderQueries != nil {
		return svcCtx.OrderQueries, nil
	}
	return nil, errors.New("order query service unavailable")
}

func backofficeOrderService(svcCtx *svc.ServiceContext) (*orderquery.BackofficeService, error) {
	if svcCtx.BackofficeOrders != nil {
		return svcCtx.BackofficeOrders, nil
	}
	return nil, errors.New("backoffice order query service unavailable")
}
