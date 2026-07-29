package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type benchmarkConfig struct {
	Name           string
	URL            string
	Requests       int
	Concurrency    int
	Timeout        time.Duration
	ExpectedStatus int
}

func runBenchmark(ctx context.Context, config benchmarkConfig) (report, error) {
	if config.URL == "" || config.Requests <= 0 || config.Concurrency <= 0 || config.Timeout <= 0 {
		return report{}, fmt.Errorf("url, requests, concurrency, and timeout must be positive")
	}
	if config.ExpectedStatus == 0 {
		config.ExpectedStatus = http.StatusOK
	}

	transport := &http.Transport{
		MaxIdleConns:        config.Concurrency * 2,
		MaxIdleConnsPerHost: config.Concurrency,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: config.Timeout}
	defer transport.CloseIdleConnections()

	jobs := make(chan struct{})
	results := make(chan requestResult, config.Requests)
	var workers sync.WaitGroup
	for index := 0; index < config.Concurrency; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range jobs {
				results <- executeRequest(ctx, client, config.URL, config.ExpectedStatus)
			}
		}()
	}

	startedAt := time.Now()
	for index := 0; index < config.Requests; index++ {
		jobs <- struct{}{}
	}
	close(jobs)
	workers.Wait()
	close(results)
	elapsed := time.Since(startedAt)

	collected := make([]requestResult, 0, config.Requests)
	for result := range results {
		collected = append(collected, result)
	}
	summary := summarize(collected, elapsed)
	summary.Name = config.Name
	summary.URL = config.URL
	return summary, nil
}

func executeRequest(ctx context.Context, client *http.Client, url string, expectedStatus int) requestResult {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return requestResult{Err: err}
	}
	request.Header.Set("Accept", "application/json")

	startedAt := time.Now()
	response, err := client.Do(request)
	latency := time.Since(startedAt)
	if err != nil {
		return requestResult{Latency: latency, Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	bytesRead, readErr := io.Copy(io.Discard, response.Body)
	result := requestResult{StatusCode: response.StatusCode, Latency: latency, Bytes: bytesRead, Err: readErr}
	if result.Err == nil && response.StatusCode != expectedStatus {
		result.Err = fmt.Errorf("unexpected status: got=%d want=%d", response.StatusCode, expectedStatus)
	}
	return result
}
