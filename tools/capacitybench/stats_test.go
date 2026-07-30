package main

import (
	"errors"
	"testing"
	"time"
)

func TestSummarizeScenarioReportsTailLatencyAndFailures(t *testing.T) {
	samples := []sample{
		{Duration: 10 * time.Millisecond, StatusCode: 200},
		{Duration: 20 * time.Millisecond, StatusCode: 200},
		{Duration: 30 * time.Millisecond, StatusCode: 409, Err: errors.New("conflict")},
		{Duration: 40 * time.Millisecond, Err: errors.New("timeout")},
	}

	got := summarizeSamples("order-cycle", samples, 2*time.Second, 0)
	if got.Attempts != 4 || got.Success != 2 || got.Failed != 2 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.SuccessRate != 0.5 || got.QPS != 2 {
		t.Fatalf("unexpected rates: %+v", got)
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
