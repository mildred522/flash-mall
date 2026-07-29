package main

import (
	"math"
	"sort"
	"strconv"
	"time"
)

type requestResult struct {
	StatusCode int
	Latency    time.Duration
	Bytes      int64
	Err        error
}

type report struct {
	Name          string         `json:"name"`
	URL           string         `json:"url"`
	Attempts      int            `json:"attempts"`
	Success       int            `json:"success"`
	Errors        int            `json:"errors"`
	SuccessRate   float64        `json:"success_rate"`
	QPS           float64        `json:"qps"`
	ElapsedMS     float64        `json:"elapsed_ms"`
	P50MS         float64        `json:"p50_ms"`
	P95MS         float64        `json:"p95_ms"`
	P99MS         float64        `json:"p99_ms"`
	MaxMS         float64        `json:"max_ms"`
	ResponseBytes int64          `json:"response_bytes"`
	StatusCodes   map[string]int `json:"status_codes"`
}

func summarize(results []requestResult, elapsed time.Duration) report {
	latencies := make([]float64, 0, len(results))
	statusCodes := make(map[string]int)
	var success, errors int
	var responseBytes int64
	for _, result := range results {
		if result.StatusCode != 0 {
			statusCodes[strconv.Itoa(result.StatusCode)]++
		}
		if result.Err != nil {
			errors++
			continue
		}
		if result.StatusCode >= 200 && result.StatusCode < 300 {
			success++
			latencies = append(latencies, float64(result.Latency.Microseconds())/1000)
			responseBytes += result.Bytes
		}
	}
	sort.Float64s(latencies)
	attempts := len(results)
	result := report{Attempts: attempts, Success: success, Errors: errors, StatusCodes: statusCodes}
	if attempts > 0 {
		result.SuccessRate = float64(success) / float64(attempts)
	}
	if elapsed > 0 {
		result.QPS = float64(attempts) / elapsed.Seconds()
		result.ElapsedMS = float64(elapsed.Microseconds()) / 1000
	}
	if len(latencies) > 0 {
		result.P50MS = percentile(latencies, 0.50)
		result.P95MS = percentile(latencies, 0.95)
		result.P99MS = percentile(latencies, 0.99)
		result.MaxMS = latencies[len(latencies)-1]
		result.ResponseBytes = responseBytes / int64(len(latencies))
	}
	return result
}

func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if p <= 0 {
		return sortedValues[0]
	}
	if p >= 1 {
		return sortedValues[len(sortedValues)-1]
	}
	index := int(math.Ceil(p*float64(len(sortedValues)))) - 1
	if index < 0 {
		index = 0
	}
	return sortedValues[index]
}
