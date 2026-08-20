package main

import (
	"errors"
	"testing"
	"time"
)

func TestSummarizeScenarioReportsTailLatencyAndFailures(t *testing.T) {
	samples := []sample{
		{Duration: 10 * time.Millisecond, StatusCode: 200, Operation: "catalog", Steps: map[string]time.Duration{"fetch": 8 * time.Millisecond}},
		{Duration: 20 * time.Millisecond, StatusCode: 200, Operation: "catalog", Steps: map[string]time.Duration{"fetch": 18 * time.Millisecond}},
		{Duration: 30 * time.Millisecond, StatusCode: 409, Err: errors.New("conflict"), Operation: "detail", Steps: map[string]time.Duration{"fetch": 28 * time.Millisecond}},
		{Duration: 40 * time.Millisecond, Err: errors.New("timeout"), Operation: "detail", Steps: map[string]time.Duration{"fetch": 38 * time.Millisecond}},
	}

	got := summarizeSamples("order-cycle", samples, 2*time.Second, 0)
	if got.Attempts != 4 || got.Success != 2 || got.Failed != 2 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.SuccessRate != 0.5 || got.QPS != 2 {
		t.Fatalf("unexpected rates: %+v", got)
	}
	if got.HTTPRequests != 4 || got.HTTPQPS != 2 {
		t.Fatalf("unexpected HTTP throughput: %+v", got)
	}
	withDrops := summarizeSamples("read", samples, 2*time.Second, 4)
	if withDrops.Attempts != 8 || withDrops.Completed != 4 || withDrops.QPS != 2 {
		t.Fatalf("dropped work must not count as completed throughput: %+v", withDrops)
	}
	if got.P50MS != 20 || got.P95MS != 40 || got.P99MS != 40 {
		t.Fatalf("unexpected percentiles: %+v", got)
	}
	if got.StatusCodes["200"] != 2 || got.StatusCodes["409"] != 1 || got.Errors["transport"] != 1 {
		t.Fatalf("unexpected classifications: %+v", got)
	}
	if len(got.ErrorSamples) != 2 || got.ErrorSamples[0] != "conflict" || got.ErrorSamples[1] != "timeout" {
		t.Fatalf("error samples=%v", got.ErrorSamples)
	}
	if got.Operations["catalog"].Samples != 2 || got.Operations["catalog"].P95MS != 20 {
		t.Fatalf("operation summaries=%+v", got.Operations)
	}
	if got.Steps["fetch"].Samples != 4 || got.Steps["fetch"].P95MS != 38 {
		t.Fatalf("step summaries=%+v", got.Steps)
	}
}

func TestEvaluateSLORequiresPerformanceAndCorrectness(t *testing.T) {
	report := scenarioReport{
		Scenario: "read", SuccessRate: 0.999, P95MS: 80, P99MS: 200,
		Invariants: invariantReport{Passed: true},
	}
	if got := evaluateSLO(report, defaultSLO("read")); !got.Passed {
		t.Fatalf("expected report to pass: %+v", got)
	}

	report.Invariants.Passed = false
	report.Invariants.Violations = []string{"duplicate_order"}
	if got := evaluateSLO(report, defaultSLO("read")); got.Passed {
		t.Fatalf("correctness violation must fail SLO: %+v", got)
	}
}

func TestDefaultSLORejectsUnknownScenario(t *testing.T) {
	if got := defaultSLO("unknown"); got.SuccessRate != 0 {
		t.Fatalf("unknown scenario should not receive an implicit SLO: %+v", got)
	}
}
