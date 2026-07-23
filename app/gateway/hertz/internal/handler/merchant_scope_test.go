package handler

import (
	"context"
	"testing"

	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"
	"flash-mall/app/gateway/hertz/internal/svc"
)

func TestSelectedMerchantIDFromServiceUsesMerchantQueryService(t *testing.T) {
	repository := &merchantScopeRepositoryStub{firstMerchantID: 1000, firstFound: true}
	svcCtx := &svc.ServiceContext{MerchantQueries: merchantquery.NewService(repository)}
	merchantID, err := selectedMerchantIDFromService(
		context.Background(), svcCtx, authctx.Identity{UserID: 1001, Role: authctx.RoleUser},
	)
	if err != nil || merchantID != 1000 {
		t.Fatalf("merchant id=%d err=%v", merchantID, err)
	}
}

type merchantScopeRepositoryStub struct {
	firstMerchantID int64
	firstFound      bool
}

func (*merchantScopeRepositoryStub) MerchantsByUser(context.Context, int64) ([]merchantquery.Merchant, error) {
	return nil, nil
}
func (*merchantScopeRepositoryStub) LatestApplication(context.Context, int64) (merchantquery.Application, bool, error) {
	return merchantquery.Application{}, false, nil
}
func (*merchantScopeRepositoryStub) Dashboard(context.Context, int64) (merchantquery.DashboardStats, error) {
	return merchantquery.DashboardStats{}, nil
}
func (r *merchantScopeRepositoryStub) FirstMerchantByUser(context.Context, int64) (int64, bool, error) {
	return r.firstMerchantID, r.firstFound, nil
}
func (*merchantScopeRepositoryStub) UserCanAccessMerchant(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (*merchantScopeRepositoryStub) DefaultActiveMerchant(context.Context) (int64, bool, error) {
	return 0, false, nil
}
