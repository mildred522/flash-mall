package main

import (
	"context"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

type countingExecutor struct{ calls atomic.Int64 }

func (e *countingExecutor) execute(_ context.Context, _ string, _ int64) sample {
	e.calls.Add(1)
	return sample{StatusCode: 200, Duration: time.Millisecond}
}

func TestRunLoadExecutesFixedRequestCount(t *testing.T) {
	executor := &countingExecutor{}
	report := runLoad(t.Context(), executor, loadConfig{
		Scenario: "read", Requests: 25, Concurrency: 4,
	})
	if report.Attempts != 25 || report.Success != 25 || executor.calls.Load() != 25 {
		t.Fatalf("unexpected report=%+v calls=%d", report, executor.calls.Load())
	}
}

func TestValidateTargetAllowsOnlyLoopbackForMutatingScenarios(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:8889", "http://localhost:8889", "http://[::1]:8889"} {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateTarget(target, "order-cycle", true); err != nil {
			t.Fatalf("target %s rejected: %v", raw, err)
		}
	}

	target, _ := url.Parse("https://mall.example.com")
	if err := validateTarget(target, "order-cycle", true); err == nil {
		t.Fatal("remote mutating target must be rejected")
	}
	if err := validateTarget(target, "read", false); err != nil {
		t.Fatalf("read-only remote target should be allowed: %v", err)
	}
}

func TestValidateTargetRequiresExplicitMutationPermission(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:8889")
	if err := validateTarget(target, "payment-cycle", false); err == nil {
		t.Fatal("mutating scenario must require explicit permission")
	}
}
