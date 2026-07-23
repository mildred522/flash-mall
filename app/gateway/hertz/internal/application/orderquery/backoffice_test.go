package orderquery

import (
	"context"
	"testing"
)

func TestBackofficeListAddsOrderStatusText(t *testing.T) {
	repository := &fakeBackofficeRepository{orders: []BackofficeOrderItem{{OrderID: "o-1", Status: 3}}, total: 1}
	service := NewBackofficeService(repository)

	result, err := service.ListAdminOrders(context.Background(), AdminListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list admin orders: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].StatusText != "shipped" {
		t.Fatalf("items = %#v", result.Items)
	}
}

func TestBackofficeRefundListAddsRefundStatusText(t *testing.T) {
	repository := &fakeBackofficeRepository{refunds: []RefundItem{{RefundID: "r-1", Status: 2}}, total: 1}
	service := NewBackofficeService(repository)

	result, err := service.ListMerchantRefunds(context.Background(), RefundListQuery{MerchantID: 7, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list merchant refunds: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].StatusText != "success" {
		t.Fatalf("items = %#v", result.Items)
	}
}

func TestBackofficeAdminRefundListUsesUnscopedRepositoryQuery(t *testing.T) {
	repository := &fakeBackofficeRepository{refunds: []RefundItem{{RefundID: "r-2", Status: 1}}, total: 1}
	result, err := NewBackofficeService(repository).ListAdminRefunds(context.Background(), RefundListQuery{
		MerchantID: 7, Page: 2, PageSize: 10, Status: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.adminRefundsCalled || result.Items[0].StatusText != "approved" {
		t.Fatalf("called=%v items=%+v", repository.adminRefundsCalled, result.Items)
	}
}

func TestBackofficeAdminDetailRejectsMissingOrder(t *testing.T) {
	service := NewBackofficeService(&fakeBackofficeRepository{})
	_, err := service.AdminDetail(context.Background(), "missing")
	if err == nil {
		t.Fatal("missing order must return an error")
	}
}

type fakeBackofficeRepository struct {
	orders             []BackofficeOrderItem
	refunds            []RefundItem
	total              int64
	adminRefundsCalled bool
}

func (f *fakeBackofficeRepository) ListOrders(context.Context, AdminListQuery) ([]BackofficeOrderItem, int64, error) {
	return f.orders, f.total, nil
}
func (f *fakeBackofficeRepository) DetailByID(context.Context, string) (Detail, bool, error) {
	return Detail{}, false, nil
}
func (f *fakeBackofficeRepository) StatusLogs(context.Context, string) ([]StatusLogItem, error) {
	return nil, nil
}
func (f *fakeBackofficeRepository) ListMerchantOrders(context.Context, MerchantListQuery) ([]BackofficeOrderItem, int64, error) {
	return f.orders, f.total, nil
}
func (f *fakeBackofficeRepository) ListMerchantRefunds(context.Context, RefundListQuery) ([]RefundItem, int64, error) {
	return f.refunds, f.total, nil
}
func (f *fakeBackofficeRepository) ListAdminRefunds(context.Context, RefundListQuery) ([]RefundItem, int64, error) {
	f.adminRefundsCalled = true
	return f.refunds, f.total, nil
}
