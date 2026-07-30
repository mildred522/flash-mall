package main

import (
	"math"
	"sort"
	"strconv"
	"time"
)

type sample struct {
	Duration   time.Duration
	StatusCode int
	Err        error
}

type invariantReport struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations,omitempty"`
}

type scenarioReport struct {
	Scenario     string          `json:"scenario"`
	RPS          int             `json:"target_rps"`
	Concurrency  int             `json:"concurrency"`
	DurationSec  float64         `json:"duration_seconds"`
	Attempts     int             `json:"attempts"`
	Success      int             `json:"success"`
	Failed       int             `json:"failed"`
	Dropped      int             `json:"dropped"`
	SuccessRate  float64         `json:"success_rate"`
	QPS          float64         `json:"qps"`
	P50MS        float64         `json:"p50_ms"`
	P95MS        float64         `json:"p95_ms"`
	P99MS        float64         `json:"p99_ms"`
	MaxMS        float64         `json:"max_ms"`
	StatusCodes  map[string]int  `json:"status_codes"`
	Errors       map[string]int  `json:"errors"`
	ErrorSamples []string        `json:"error_samples,omitempty"`
	Invariants   invariantReport `json:"invariants"`
	SLO          sloResult       `json:"slo"`
}

type sloTarget struct {
	SuccessRate float64 `json:"minimum_success_rate"`
	P95MS       float64 `json:"maximum_p95_ms"`
	P99MS       float64 `json:"maximum_p99_ms"`
}

type sloResult struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations,omitempty"`
}

func summarizeSamples(scenario string, samples []sample, elapsed time.Duration, dropped int) scenarioReport {
	report := scenarioReport{
		Scenario: scenario, Dropped: dropped, Attempts: len(samples) + dropped,
		StatusCodes: map[string]int{}, Errors: map[string]int{},
		Invariants: invariantReport{Passed: scenario == "read"},
	}
	if scenario != "read" {
		report.Invariants.Violations = []string{"external_verification_required"}
	}
	latencies := make([]float64, 0, len(samples))
	for _, item := range samples {
		if item.StatusCode > 0 {
			report.StatusCodes[strconv.Itoa(item.StatusCode)]++
		}
		latencies = append(latencies, float64(item.Duration.Microseconds())/1000)
		if item.Err == nil && item.StatusCode >= 200 && item.StatusCode < 300 {
			report.Success++
			continue
		}
		report.Failed++
		if len(report.ErrorSamples) < 5 && item.Err != nil {
			report.ErrorSamples = append(report.ErrorSamples, item.Err.Error())
		}
		if item.StatusCode == 0 {
			report.Errors["transport"]++
		} else {
			report.Errors["http"]++
		}
	}
	report.Failed += dropped
	if report.Attempts > 0 {
		report.SuccessRate = float64(report.Success) / float64(report.Attempts)
	}
	if elapsed > 0 {
		report.QPS = float64(report.Attempts) / elapsed.Seconds()
		report.DurationSec = elapsed.Seconds()
	}
	sort.Float64s(latencies)
	if len(latencies) > 0 {
		report.P50MS = percentile(latencies, 0.50)
		report.P95MS = percentile(latencies, 0.95)
		report.P99MS = percentile(latencies, 0.99)
		report.MaxMS = latencies[len(latencies)-1]
	}
	return report
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func defaultSLO(scenario string) sloTarget {
	switch scenario {
	case "read":
		return sloTarget{SuccessRate: 0.999, P95MS: 100, P99MS: 250}
	case "order-cycle":
		return sloTarget{SuccessRate: 0.99, P95MS: 1500, P99MS: 3000}
	case "payment-cycle", "idempotency":
		return sloTarget{SuccessRate: 0.99, P95MS: 2000, P99MS: 4000}
	default:
		return sloTarget{}
	}
}

func evaluateSLO(report scenarioReport, target sloTarget) sloResult {
	result := sloResult{Passed: true}
	if target.SuccessRate == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, "unknown_scenario")
	}
	if report.SuccessRate < target.SuccessRate {
		result.Passed = false
		result.Violations = append(result.Violations, "success_rate")
	}
	if report.P95MS > target.P95MS {
		result.Passed = false
		result.Violations = append(result.Violations, "p95")
	}
	if report.P99MS > target.P99MS {
		result.Passed = false
		result.Violations = append(result.Violations, "p99")
	}
	if !report.Invariants.Passed {
		result.Passed = false
		result.Violations = append(result.Violations, report.Invariants.Violations...)
	}
	return result
}
