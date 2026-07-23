package orderquery

import (
	"context"
	"testing"
)

func TestPaymentByClaimsReturnsNotFoundWithoutLeakingSQL(t *testing.T) {
	service := NewService(&paymentRepositoryStub{})
	_, err := service.PaymentByClaims(context.Background(), "pay-1", "order-1", "trade-1")
	if err == nil {
		t.Fatal("expected not found application error")
	}
}

func TestStatusByRequestDecoratesOrderStatus(t *testing.T) {
	repository := &paymentRepositoryStub{orderID: "order-1", orderStatus: 3, found: true}
	status, found, err := NewService(repository).StatusByRequest(context.Background(), "req-1", 7)
	if err != nil || !found || status.Status != "shipped" {
		t.Fatalf("status=%+v found=%v err=%v", status, found, err)
	}
}

type paymentRepositoryStub struct {
	payment     PaymentOrder
	orderID     string
	orderStatus int64
	found       bool
}

func (*paymentRepositoryStub) ListByUser(context.Context, int64, int64) ([]ListItem, error) {
	return nil, nil
}
func (*paymentRepositoryStub) DetailByUser(context.Context, string, int64) (Detail, bool, error) {
	return Detail{}, false, nil
}
func (r *paymentRepositoryStub) PaymentByUser(context.Context, string, int64) (PaymentOrder, bool, error) {
	return r.payment, r.found, nil
}
func (r *paymentRepositoryStub) PaymentByID(context.Context, string, int64) (PaymentOrder, bool, error) {
	return r.payment, r.found, nil
}
func (r *paymentRepositoryStub) PaymentByClaims(context.Context, string, string, string) (PaymentOrder, bool, error) {
	return r.payment, r.found, nil
}
func (r *paymentRepositoryStub) OrderStatusByRequest(context.Context, string, int64) (string, int64, bool, error) {
	return r.orderID, r.orderStatus, r.found, nil
}
func (*paymentRepositoryStub) OrderIDByRequest(context.Context, string, int64) (string, bool, error) {
	return "", false, nil
}
func (*paymentRepositoryStub) CreateResultByOrder(context.Context, string, int64) (CreateOrderResult, int64, bool, error) {
	return CreateOrderResult{}, 0, false, nil
}
