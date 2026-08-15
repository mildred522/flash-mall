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
	Operation  string
	Steps      map[string]time.Duration
}

type invariantReport struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations,omitempty"`
}

type scenarioReport struct {
	Scenario     string                    `json:"scenario"`
	RPS          int                       `json:"target_rps"`
	Concurrency  int                       `json:"concurrency"`
	DurationSec  float64                   `json:"duration_seconds"`
	Attempts     int                       `json:"attempts"`
	Completed    int                       `json:"completed"`
	Success      int                       `json:"success"`
	Failed       int                       `json:"failed"`
	Dropped      int                       `json:"dropped"`
	SuccessRate  float64                   `json:"success_rate"`
	QPS          float64                   `json:"qps"`
	P50MS        float64                   `json:"p50_ms"`
	P95MS        float64                   `json:"p95_ms"`
	P99MS        float64                   `json:"p99_ms"`
	MaxMS        float64                   `json:"max_ms"`
	StatusCodes  map[string]int            `json:"status_codes"`
	Errors       map[string]int            `json:"errors"`
	ErrorSamples []string                  `json:"error_samples,omitempty"`
	Operations   map[string]latencySummary `json:"operations,omitempty"`
	Steps        map[string]latencySummary `json:"steps,omitempty"`
	Invariants   invariantReport           `json:"invariants"`
	SLO          sloResult                 `json:"slo"`
}

type latencySummary struct {
	Samples int     `json:"samples"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	P99MS   float64 `json:"p99_ms"`
	MaxMS   float64 `json:"max_ms"`
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
		Scenario:    scenario,
		Dropped:     dropped,
		Attempts:    len(samples) + dropped,
		Completed:   len(samples),
		StatusCodes: map[string]int{}, Errors: map[string]int{},
		Operations: map[string]latencySummary{}, Steps: map[string]latencySummary{},
		Invariants: invariantReport{Passed: scenario == "read"},
	}
	if scenario != "read" {
		report.Invariants.Violations = []string{"external_verification_required"}
	}
	latencies := make([]float64, 0, len(samples))
	operationLatencies := map[string][]float64{}
	stepLatencies := map[string][]float64{}
	for _, item := range samples {
		if item.StatusCode > 0 {
			report.StatusCodes[strconv.Itoa(item.StatusCode)]++
		}
		latencyMS := durationMilliseconds(item.Duration)
		latencies = append(latencies, latencyMS)
		if item.Operation != "" {
			operationLatencies[item.Operation] = append(operationLatencies[item.Operation], latencyMS)
		}
		for name, duration := range item.Steps {
			stepLatencies[name] = append(stepLatencies[name], durationMilliseconds(duration))
		}
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
		report.QPS = float64(report.Completed) / elapsed.Seconds()
		report.DurationSec = elapsed.Seconds()
	}
	sort.Float64s(latencies)
	if len(latencies) > 0 {
		summary := summarizeLatencies(latencies)
		report.P50MS = summary.P50MS
		report.P95MS = summary.P95MS
		report.P99MS = summary.P99MS
		report.MaxMS = summary.MaxMS
	}
	for name, values := range operationLatencies {
		report.Operations[name] = summarizeLatencies(values)
	}
	for name, values := range stepLatencies {
		report.Steps[name] = summarizeLatencies(values)
	}
	return report
}

func durationMilliseconds(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000
}

func summarizeLatencies(values []float64) latencySummary {
	sort.Float64s(values)
	return latencySummary{
		Samples: len(values),
		P50MS:   percentile(values, 0.50),
		P95MS:   percentile(values, 0.95),
		P99MS:   percentile(values, 0.99),
		MaxMS:   values[len(values)-1],
	}
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
