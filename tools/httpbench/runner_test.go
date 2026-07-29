package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunBenchmarkCountsFixedSuccessfulResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[1,2,3]}`))
	}))
	defer server.Close()

	got, err := runBenchmark(context.Background(), benchmarkConfig{
		Name:           "test",
		URL:            server.URL,
		Requests:       12,
		Concurrency:    3,
		Timeout:        time.Second,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempts != 12 || got.Success != 12 || got.Errors != 0 {
		t.Fatalf("unexpected report: %+v", got)
	}
	if got.ResponseBytes != int64(len(`{"items":[1,2,3]}`)) {
		t.Fatalf("response bytes=%d", got.ResponseBytes)
	}
}

func TestRunBenchmarkRejectsInvalidConfiguration(t *testing.T) {
	_, err := runBenchmark(context.Background(), benchmarkConfig{})
	if err == nil {
		t.Fatal("expected invalid configuration error")
	}
}
