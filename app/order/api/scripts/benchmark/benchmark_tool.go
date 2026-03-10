package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// CreateOrderReq mirrors types.CreateOrderReq in order-api.
type CreateOrderReq struct {
	RequestId string `json:"request_id"`
	UserId    int64  `json:"user_id"`
	ProductId int64  `json:"product_id"`
	Amount    int64  `json:"amount"`
}

type phaseStats struct {
	Success      int64            `json:"success"`
	Failed       int64            `json:"failed"`
	TotalLatency int64            `json:"-"`
	StatusCodes  map[string]int64 `json:"status_codes"`
	ErrorTypes   map[string]int64 `json:"error_types"`
	latencies    []int64
	mu           sync.Mutex
}

func newPhaseStats() *phaseStats {
	return &phaseStats{
		StatusCodes: make(map[string]int64),
		ErrorTypes:  make(map[string]int64),
	}
}

func (s *phaseStats) record(statusCode int, err error, latencyMicro int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err == nil && statusCode == http.StatusOK {
		s.Success++
	} else {
		s.Failed++
	}
	s.TotalLatency += latencyMicro
	s.latencies = append(s.latencies, latencyMicro)

	if statusCode > 0 {
		key := strconv.Itoa(statusCode)
		s.StatusCodes[key]++
	}

	if err != nil {
		s.ErrorTypes[classifyError(err)]++
	}
}

func (s *phaseStats) snapshot() (success int64, failed int64, totalLatency int64, latencies []int64, statusCodes map[string]int64, errorTypes map[string]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	success = s.Success
	failed = s.Failed
	totalLatency = s.TotalLatency
	latencies = append([]int64(nil), s.latencies...)

	statusCodes = make(map[string]int64, len(s.StatusCodes))
	for k, v := range s.StatusCodes {
		statusCodes[k] = v
	}

	errorTypes = make(map[string]int64, len(s.ErrorTypes))
	for k, v := range s.ErrorTypes {
		errorTypes[k] = v
	}

	return
}

var (
	url             string
	concurrency     int
	requests        int
	duration        int
	warmup          int
	scenario        string
	reportPath      string
	fixedRequestID  string
	userID          int64
	productID       int64
	amount          int64
	timeoutMs       int
	targetRPS       int
	maxErrorSamples int64
)

func init() {
	flag.StringVar(&url, "url", "http://localhost:8888/api/order/create", "API URL")
	flag.IntVar(&concurrency, "c", 10, "Concurrency level")
	flag.IntVar(&requests, "n", 0, "Measured request count (0 means duration mode)")
	flag.IntVar(&duration, "d", 10, "Measurement duration in seconds (used when n=0)")
	flag.IntVar(&warmup, "warmup", 0, "Warmup duration in seconds (not included in report metrics)")
	flag.StringVar(&scenario, "scenario", "seckill", "Scenario label")
	flag.StringVar(&reportPath, "report", "", "Output report JSON path")
	flag.StringVar(&fixedRequestID, "fixed-request-id", "", "Use same request_id for all requests (idempotency test)")
	flag.Int64Var(&userID, "user", 1, "User id")
	flag.Int64Var(&productID, "product", 100, "Product id")
	flag.Int64Var(&amount, "amount", 1, "Order amount")
	flag.IntVar(&timeoutMs, "timeout-ms", 5000, "Per-request timeout in milliseconds")
	flag.IntVar(&targetRPS, "rps", 0, "Global target request rate (0 means closed-loop)")
	flag.Int64Var(&maxErrorSamples, "max-error-samples", 5, "Max failed request samples to print")
}

func main() {
	flag.Parse()

	if concurrency <= 0 {
		fmt.Fprintln(os.Stderr, "concurrency must be > 0")
		os.Exit(1)
	}
	if duration <= 0 {
		duration = 10
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	if warmup < 0 {
		warmup = 0
	}

	mode := "duration"
	if requests > 0 {
		mode = "requests"
	}
	loadModel := "closed_loop"
	if targetRPS > 0 {
		loadModel = "open_loop"
	}

	fmt.Printf("Starting benchmark: URL=%s, Concurrency=%d\n", url, concurrency)
	fmt.Printf("Scenario: %s\n", scenario)
	fmt.Printf("Mode: %s, Warmup=%ds\n", mode, warmup)
	if requests > 0 {
		fmt.Printf("Target measured requests: %d\n", requests)
	} else {
		fmt.Printf("Measurement duration: %ds\n", duration)
	}
	if targetRPS > 0 {
		fmt.Printf("Load model: %s, Target RPS=%d\n", loadModel, targetRPS)
	} else {
		fmt.Printf("Load model: %s\n", loadModel)
	}

	measureStats := newPhaseStats()
	var warmupRequests int64
	var measuredStarted int64
	var measuredFinished int64
	var printedErrors int64

	runStart := time.Now()
	measureStart := runStart.Add(time.Duration(warmup) * time.Second)

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			close(stopCh)
		})
	}

	if requests == 0 {
		totalRunSeconds := warmup + duration
		time.AfterFunc(time.Duration(totalRunSeconds)*time.Second, stop)
	}

	var loadLimiter *rate.Limiter
	if targetRPS > 0 {
		burst := targetRPS
		if burst < 1 {
			burst = 1
		}
		loadLimiter = rate.NewLimiter(rate.Limit(targetRPS), burst)
	}

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}

		for {
			select {
			case <-stopCh:
				return
			default:
			}

			if loadLimiter != nil {
				if err := loadLimiter.Wait(context.Background()); err != nil {
					return
				}
			}

			now := time.Now()
			measuring := !now.Before(measureStart)
			if !measuring {
				atomic.AddInt64(&warmupRequests, 1)
			}

			if requests > 0 && measuring {
				idx := atomic.AddInt64(&measuredStarted, 1)
				if idx > int64(requests) {
					return
				}
			}

			reqBody := CreateOrderReq{
				UserId:    userID,
				ProductId: productID,
				Amount:    amount,
			}
			if fixedRequestID != "" {
				reqBody.RequestId = fixedRequestID
			} else {
				reqBody.RequestId = uuid.NewString()
			}
			payload, _ := json.Marshal(reqBody)

			started := time.Now()
			resp, err := client.Post(url, "application/json", bytes.NewBuffer(payload))
			latencyMicro := time.Since(started).Microseconds()

			statusCode := 0
			if resp != nil {
				statusCode = resp.StatusCode
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}

			if measuring {
				measureStats.record(statusCode, err, latencyMicro)
				if requests > 0 {
					finished := atomic.AddInt64(&measuredFinished, 1)
					if finished >= int64(requests) {
						stop()
					}
				}
			}

			if err != nil || statusCode != http.StatusOK {
				sampleIdx := atomic.AddInt64(&printedErrors, 1)
				if sampleIdx <= maxErrorSamples {
					if err != nil {
						fmt.Printf("Sample failure #%d: error=%v\n", sampleIdx, err)
					} else {
						fmt.Printf("Sample failure #%d: status=%d\n", sampleIdx, statusCode)
					}
				}
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}
	wg.Wait()

	runEnd := time.Now()
	measuredElapsed := runEnd.Sub(measureStart)
	if measuredElapsed < 0 {
		measuredElapsed = 0
	}

	success, failed, totalLatency, latencies, statusCodes, errorTypes := measureStats.snapshot()
	totalMeasured := success + failed

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := float64(percentile(latencies, 0.50)) / 1000.0
	p95 := float64(percentile(latencies, 0.95)) / 1000.0
	p99 := float64(percentile(latencies, 0.99)) / 1000.0

	fmt.Println("\n--- Benchmark Results (measured window only) ---")
	fmt.Printf("Total runtime: %v\n", runEnd.Sub(runStart))
	fmt.Printf("Measured window: %v\n", measuredElapsed)
	fmt.Printf("Warmup requests: %d\n", atomic.LoadInt64(&warmupRequests))
	fmt.Printf("Measured requests: %d\n", totalMeasured)
	fmt.Printf("Success: %d\n", success)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.2f%%\n", ratio(success, totalMeasured)*100)
	fmt.Printf("QPS: %.2f\n", qps(totalMeasured, measuredElapsed))
	fmt.Printf("Avg Latency: %.2f ms\n", avgLatency(totalLatency, totalMeasured))
	fmt.Printf("P50: %.2f ms, P95: %.2f ms, P99: %.2f ms\n", p50, p95, p99)
	printSortedMap("Status breakdown", statusCodes)
	printSortedMap("Error breakdown", errorTypes)

	if reportPath != "" {
		report := map[string]any{
			"scenario":            scenario,
			"url":                 url,
			"concurrency":         concurrency,
			"mode":                mode,
			"load_model":          loadModel,
			"target_rps":          targetRPS,
			"requests":            requests,
			"warmup_seconds":      warmup,
			"measurement_seconds": duration,
			"timeout_ms":          timeoutMs,
			"started_at":          runStart.Format(time.RFC3339),
			"ended_at":            runEnd.Format(time.RFC3339),
			"runtime_seconds":     runEnd.Sub(runStart).Seconds(),
			"measured_requests":   totalMeasured,
			"warmup_requests":     atomic.LoadInt64(&warmupRequests),
			"success":             success,
			"failed":              failed,
			"success_rate":        ratio(success, totalMeasured),
			"error_rate":          ratio(failed, totalMeasured),
			"qps":                 qps(totalMeasured, measuredElapsed),
			"avg_ms":              avgLatency(totalLatency, totalMeasured),
			"p50_ms":              p50,
			"p95_ms":              p95,
			"p99_ms":              p99,
			"status_codes":        statusCodes,
			"error_types":         errorTypes,
			"fixed_request":       fixedRequestID != "",
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal report: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(reportPath, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Report written: %s\n", reportPath)
	}
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "network"
}

func printSortedMap(title string, m map[string]int64) {
	fmt.Printf("%s:\n", title)
	if len(m) == 0 {
		fmt.Println("  (none)")
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s => %d\n", k, m[k])
	}
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func ratio(part int64, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func qps(total int64, elapsed time.Duration) float64 {
	if elapsed.Seconds() <= 0 {
		return 0
	}
	return float64(total) / elapsed.Seconds()
}

func avgLatency(totalMicro int64, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(totalMicro) / float64(total) / 1000.0
}
