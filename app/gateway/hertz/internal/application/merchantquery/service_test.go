package merchantquery

import (
	"context"
	"testing"
)

func TestLatestApplicationAddsStatusText(t *testing.T) {
	repository := &fakeRepository{
		application: Application{ApplyID: 12, Status: 2},
		found:       true,
	}
	service := NewService(repository)

	response, err := service.LatestApplication(context.Background(), 1001)
	if err != nil {
		t.Fatalf("latest application: %v", err)
	}
	if response.Application == nil || response.Application.StatusText != "rejected" {
		t.Fatalf("application = %+v, want rejected status text", response.Application)
	}
}

func TestLatestApplicationReturnsNullWhenMissing(t *testing.T) {
	service := NewService(&fakeRepository{})

	response, err := service.LatestApplication(context.Background(), 1001)
	if err != nil {
		t.Fatalf("latest application: %v", err)
	}
	if response.Application != nil {
		t.Fatalf("application = %+v, want nil", response.Application)
	}
}

func TestResolveScopeRejectsMerchantOutsideUserMembership(t *testing.T) {
	repository := &fakeRepository{canAccess: false}
	_, err := NewService(repository).ResolveScope(context.Background(), ScopeRequest{UserID: 1001, MerchantID: 8})
	if err == nil {
		t.Fatal("expected forbidden error")
	}
}

func TestResolveScopeUsesDefaultMerchantForAdmin(t *testing.T) {
	repository := &fakeRepository{defaultMerchantID: 9, defaultFound: true}
	merchantID, err := NewService(repository).ResolveScope(context.Background(), ScopeRequest{UserID: 1, Admin: true})
	if err != nil || merchantID != 9 {
		t.Fatalf("merchantID=%d err=%v", merchantID, err)
	}
}

type fakeRepository struct {
	application       Application
	found             bool
	canAccess         bool
	defaultMerchantID int64
	defaultFound      bool
}

func (f *fakeRepository) MerchantsByUser(context.Context, int64) ([]Merchant, error) {
	return nil, nil
}

func (f *fakeRepository) LatestApplication(context.Context, int64) (Application, bool, error) {
	return f.application, f.found, nil
}

func (f *fakeRepository) Dashboard(context.Context, int64) (DashboardStats, error) {
	return DashboardStats{}, nil
}

func (f *fakeRepository) FirstMerchantByUser(context.Context, int64) (int64, bool, error) {
	return 0, false, nil
}

func (f *fakeRepository) UserCanAccessMerchant(context.Context, int64, int64) (bool, error) {
	return f.canAccess, nil
}

func (f *fakeRepository) DefaultActiveMerchant(context.Context) (int64, bool, error) {
	return f.defaultMerchantID, f.defaultFound, nil
}
