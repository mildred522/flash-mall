package job

import "testing"

func TestOrderPaidEventUsesStableIDAndRoute(t *testing.T) {
	event := NewOrderPaidEvent("order-1")
	if event.EventID != "order.paid:order-1" || event.EventType != "order.paid" || event.OrderID != "order-1" {
		t.Fatalf("unexpected paid event: %+v", event)
	}
}
