package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/prometheus/client_golang/prometheus"
)

func TestShowcaseMetricsRecordReadsLayoutPublishCandidatesAndStores(t *testing.T) {
	readBefore, _ := gatheredMetric(t, "flashmall_showcase_read_total", map[string]string{"audience": "public", "result": "success"})
	publishBefore, _ := gatheredMetric(t, "flashmall_showcase_publish_total", map[string]string{"result": "success"})
	candidateBefore, _ := gatheredMetric(t, "flashmall_showcase_candidate_total", map[string]string{"result": "success"})
	_, candidateDurationBefore := gatheredMetric(t, "flashmall_showcase_candidate_duration_seconds", nil)
	_, storeDurationBefore := gatheredMetric(t, "flashmall_store_request_duration_seconds", map[string]string{"operation": "detail", "result": "success"})
	_, requestDurationBefore := gatheredMetric(t, "flashmall_showcase_request_duration_seconds", map[string]string{"audience": "public", "result": "success"})

	layout := ShowcaseResp{Items: []ShowcaseSlot{
		{SlotNo: 1, ProductID: 100, Valid: true},
		{SlotNo: 2, ProductID: 101, Valid: true},
		{SlotNo: 3, ProductID: 102, Valid: false, InvalidReason: "out_of_stock"},
	}}
	recordShowcaseRead("public", "success", layout, 15*time.Millisecond)
	recordShowcasePublish("success")
	recordShowcaseCandidate("success", 20*time.Millisecond)
	recordStoreRequest("detail", "success", 12*time.Millisecond)

	if got, _ := gatheredMetric(t, "flashmall_showcase_read_total", map[string]string{"audience": "public", "result": "success"}); got != readBefore+1 {
		t.Fatalf("read counter=%v want=%v", got, readBefore+1)
	}
	if got, _ := gatheredMetric(t, "flashmall_showcase_publish_total", map[string]string{"result": "success"}); got != publishBefore+1 {
		t.Fatalf("publish counter=%v want=%v", got, publishBefore+1)
	}
	if got, _ := gatheredMetric(t, "flashmall_showcase_candidate_total", map[string]string{"result": "success"}); got != candidateBefore+1 {
		t.Fatalf("candidate counter=%v want=%v", got, candidateBefore+1)
	}
	if got, _ := gatheredMetric(t, "flashmall_showcase_active_items", nil); got != 2 {
		t.Fatalf("active items=%v want=2", got)
	}
	if got, _ := gatheredMetric(t, "flashmall_showcase_pending_slots", nil); got != 10 {
		t.Fatalf("pending slots=%v want=10", got)
	}
	if got, _ := gatheredMetric(t, "flashmall_showcase_invalid_slots", map[string]string{"reason": "out_of_stock"}); got != 1 {
		t.Fatalf("out_of_stock slots=%v want=1", got)
	}
	if _, got := gatheredMetric(t, "flashmall_showcase_candidate_duration_seconds", nil); got != candidateDurationBefore+1 {
		t.Fatalf("candidate duration samples=%d want=%d", got, candidateDurationBefore+1)
	}
	if _, got := gatheredMetric(t, "flashmall_store_request_duration_seconds", map[string]string{"operation": "detail", "result": "success"}); got != storeDurationBefore+1 {
		t.Fatalf("store duration samples=%d want=%d", got, storeDurationBefore+1)
	}
	if _, got := gatheredMetric(t, "flashmall_showcase_request_duration_seconds", map[string]string{"audience": "public", "result": "success"}); got != requestDurationBefore+1 {
		t.Fatalf("showcase duration samples=%d want=%d", got, requestDurationBefore+1)
	}
}

func TestShowcaseMetricsExposeOnlyBoundedLabels(t *testing.T) {
	for _, result := range []string{"success", "conflict", "invalid", "error"} {
		recordShowcasePublish(result)
	}
	for _, reason := range showcaseInvalidReasons {
		if reason == "" {
			t.Fatal("invalid reason labels must be fixed non-empty values")
		}
	}
}

func TestShowcaseMetricsHandlerExposesPrometheusText(t *testing.T) {
	c := app.NewContext(0)
	MetricsHandler()(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusOK {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
	contentType := string(c.Response.Header.ContentType())
	if !strings.Contains(contentType, "text/plain") || !strings.Contains(contentType, "version=0.0.4") {
		t.Fatalf("unexpected content type %q", contentType)
	}
	body := string(c.Response.Body())
	for _, metric := range []string{
		"flashmall_showcase_read_total",
		"flashmall_showcase_publish_total",
		"flashmall_showcase_invalid_slots",
		"flashmall_store_request_duration_seconds",
	} {
		if !strings.Contains(body, metric) {
			t.Errorf("metrics response missing %s", metric)
		}
	}
	for _, forbidden := range []string{"product_id=", "merchant_id=", "operator_id=", "authorization="} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("metrics response contains high-cardinality or sensitive label %q", forbidden)
		}
	}
}

func gatheredMetric(t *testing.T, name string, labels map[string]string) (float64, uint64) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			matched := true
			for key, value := range labels {
				found := false
				for _, pair := range metric.Label {
					if pair.GetName() == key && pair.GetValue() == value {
						found = true
						break
					}
				}
				if !found {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			switch {
			case metric.Counter != nil:
				return metric.Counter.GetValue(), 0
			case metric.Gauge != nil:
				return metric.Gauge.GetValue(), 0
			case metric.Histogram != nil:
				return metric.Histogram.GetSampleSum(), metric.Histogram.GetSampleCount()
			}
		}
	}
	t.Fatalf("metric %s with labels %#v not found", name, labels)
	return 0, 0
}
