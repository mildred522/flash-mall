package handler

import (
	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
)

func requireProductInventoryInitializer(svcCtx *svc.ServiceContext) (ports.ProductInventoryInitializer, error) {
	if svcCtx.ProductInventory == nil {
		return nil, apperror.New(apperror.CodeInternal, "product inventory initializer is not configured")
	}
	return svcCtx.ProductInventory, nil
}

func requireProductSnapshotStore(svcCtx *svc.ServiceContext) (ports.ProductSnapshotStore, error) {
	if svcCtx.ProductSnapshots == nil {
		return nil, apperror.New(apperror.CodeInternal, "product snapshot store is not configured")
	}
	return svcCtx.ProductSnapshots, nil
}
