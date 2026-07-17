package handler

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestRecordPaymentCallbackUsesBoundedResult(t *testing.T) {
	counter := paymentCallbackTotal.WithLabelValues("success")
	beforeMetric := &dto.Metric{}
	if err := counter.Write(beforeMetric); err != nil {
		t.Fatal(err)
	}
	before := beforeMetric.GetCounter().GetValue()
	recordPaymentCallback("success", 20*time.Millisecond)
	afterMetric := &dto.Metric{}
	if err := counter.Write(afterMetric); err != nil {
		t.Fatal(err)
	}
	after := afterMetric.GetCounter().GetValue()
	if after-before != 1 {
		t.Fatalf("payment callback counter delta=%v", after-before)
	}
}
