package main

import (
	"errors"
	"testing"
	"time"
)

func TestSummarizeUsesSuccessfulResponsesForLatency(t *testing.T) {
	results := []requestResult{
		{StatusCode: 200, Latency: 10 * time.Millisecond},
		{StatusCode: 200, Latency: 20 * time.Millisecond},
		{StatusCode: 200, Latency: 30 * time.Millisecond},
		{StatusCode: 200, Latency: 40 * time.Millisecond},
		{Err: errors.New("timeout"), Latency: time.Second},
	}

	got := summarize(results, 2*time.Second)
	if got.Attempts != 5 || got.Success != 4 || got.Errors != 1 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.StatusCodes["200"] != 4 || got.SuccessRate != 0.8 || got.QPS != 2.5 {
		t.Fatalf("unexpected rates: %+v", got)
	}
	if got.P50MS != 20 || got.P95MS != 40 || got.P99MS != 40 || got.MaxMS != 40 {
		t.Fatalf("unexpected latency summary: %+v", got)
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	for _, tc := range []struct {
		p    float64
		want float64
	}{{0, 1}, {0.5, 3}, {0.95, 5}, {1, 5}} {
		if got := percentile(values, tc.p); got != tc.want {
			t.Fatalf("percentile(%v, %v)=%v want=%v", values, tc.p, got, tc.want)
		}
	}
}

func TestSummarizePreservesHTTPStatusForFailedExpectation(t *testing.T) {
	got := summarize([]requestResult{{
		StatusCode: 503,
		Latency:    3 * time.Millisecond,
		Err:        errors.New("unexpected status"),
	}}, time.Second)

	if got.Errors != 1 || got.Success != 0 || got.StatusCodes["503"] != 1 {
		t.Fatalf("unexpected failed response summary: %+v", got)
	}
}
