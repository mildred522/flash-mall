package handler

import (
	"errors"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestRecordOrderStageUsesBoundedLabels(t *testing.T) {
	counter := orderStageTotal.WithLabelValues(orderStageSagaSubmit, "error")
	beforeMetric := &dto.Metric{}
	if err := counter.Write(beforeMetric); err != nil {
		t.Fatal(err)
	}

	recordOrderStage(orderStageSagaSubmit, time.Now().Add(-time.Millisecond), errors.New("unavailable"))

	afterMetric := &dto.Metric{}
	if err := counter.Write(afterMetric); err != nil {
		t.Fatal(err)
	}
	if delta := afterMetric.GetCounter().GetValue() - beforeMetric.GetCounter().GetValue(); delta != 1 {
		t.Fatalf("order stage counter delta=%v", delta)
	}
}

func TestGenerateDtmGIDConvertsClientPanic(t *testing.T) {
	_, err := generateDtmGID("invalid-dtm-target")
	if err == nil {
		t.Fatal("expected invalid DTM target to return an error")
	}
}
