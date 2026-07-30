package main

import (
	"context"
	"testing"

	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"
	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/repository"
	"flash-mall/app/inventory/service"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestReserveStockCreatesBusinessSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	svc := service.New(repository.NewMemoryStockRepository(), 4)
	if err := svc.SeedStock(context.Background(), 201, 10, 4); err != nil {
		t.Fatalf("seed stock: %v", err)
	}
	handler := NewInventoryServiceImpl(svc, newInventoryMetrics(prometheus.NewRegistry()))
	requestID := "reserve-request-1"
	businessTraceID := "business-trace-1"
	_, err := handler.ReserveStock(context.Background(), &inventory.ReserveStockRequest{
		Meta: &common.RequestMeta{
			RequestId: &requestID,
			TraceId:   &businessTraceID,
		},
		OrderId:   "order-1",
		ProductId: 201,
		Quantity:  2,
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("span count=%d want=1", len(spans))
	}
	if got := spans[0].Name(); got != "inventory.reserve_stock" {
		t.Fatalf("span name=%q", got)
	}
	attributes := map[string]string{}
	for _, item := range spans[0].Attributes() {
		attributes[string(item.Key)] = item.Value.Emit()
	}
	if attributes["order.id"] != "order-1" {
		t.Fatalf("order id attribute=%q", attributes["order.id"])
	}
	if attributes["flashmall.request_id"] != requestID {
		t.Fatalf("request id attribute=%q", attributes["flashmall.request_id"])
	}
	if attributes["flashmall.business_trace_id"] != businessTraceID {
		t.Fatalf("business trace id attribute=%q", attributes["flashmall.business_trace_id"])
	}
}
