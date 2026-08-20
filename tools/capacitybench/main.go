package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

type capacityResult struct {
	SchemaVersion int            `json:"schema_version"`
	RecordedAt    string         `json:"recorded_at"`
	BaseURL       string         `json:"base_url"`
	GoVersion     string         `json:"go_version"`
	TestKind      string         `json:"test_kind,omitempty"`
	Stage         string         `json:"stage,omitempty"`
	Report        scenarioReport `json:"report"`
}

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:8889", "Hertz gateway base URL")
	scenario := flag.String("scenario", "read", "read, order-cycle, trade-cycle, payment-cycle, or idempotency")
	requests := flag.Int("requests", 0, "fixed request count; zero uses duration")
	duration := flag.Duration("duration", 30*time.Second, "measurement duration")
	warmup := flag.Duration("warmup", 5*time.Second, "warmup duration")
	rps := flag.Int("rps", 0, "target request rate; zero uses closed-loop workers")
	concurrency := flag.Int("concurrency", 20, "maximum workers")
	phone := flag.String("phone", envOr("FLASH_MALL_CAPACITY_PHONE", "13800000001"), "demo user phone")
	password := flag.String("password", envOr("FLASH_MALL_CAPACITY_PASSWORD", "flashmall123"), "demo user password")
	productID := flag.Int64("product", 100, "capacity fixture product ID")
	output := flag.String("out", "", "optional JSON report path")
	testKind := flag.String("test-kind", "", "suite category: baseline, load, stress, stability, or recovery")
	stage := flag.String("stage", "", "stable stage identifier for resource correlation")
	allowMutation := flag.Bool("allow-mutation", false, "allow local write scenarios")
	allowCompose := flag.Bool("allow-compose-target", false, "allow the fixed hertz-gateway:8889 Compose target")
	enforceSLO := flag.Bool("enforce-slo", false, "exit non-zero when the scenario SLO fails")
	flag.Parse()

	err := run(*baseURL, *scenario, *requests, *duration, *warmup, *rps, *concurrency,
		*phone, *password, *productID, *output, *testKind, *stage, *allowMutation, *allowCompose, *enforceSLO)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(
	baseURL string,
	scenario string,
	requests int,
	duration time.Duration,
	warmup time.Duration,
	rps int,
	concurrency int,
	phone string,
	password string,
	productID int64,
	output string,
	testKind string,
	stage string,
	allowMutation bool,
	allowCompose bool,
	enforceSLO bool,
) error {
	target, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return err
	}
	if err := validateTarget(target, scenario, allowMutation, allowCompose); err != nil {
		return err
	}
	switch scenario {
	case "read", "order-cycle", "trade-cycle", "payment-cycle", "idempotency":
	default:
		return fmt.Errorf("unsupported scenario %q", scenario)
	}
	if concurrency <= 0 || requests < 0 || rps < 0 || productID <= 0 {
		return fmt.Errorf("concurrency and product must be positive; requests and rps cannot be negative")
	}

	client := newBusinessClient(target.String(), phone, password, productID)
	ctx := context.Background()
	if scenario != "read" {
		if err := client.authenticate(ctx); err != nil {
			return err
		}
	}
	if warmup > 0 {
		_ = runLoad(ctx, client, loadConfig{
			Scenario: scenario, Duration: warmup, RPS: rps, Concurrency: concurrency,
		})
	}
	report := runLoad(ctx, client, loadConfig{
		Scenario: scenario, Requests: requests, Duration: duration, RPS: rps, Concurrency: concurrency,
	})
	result := capacityResult{
		SchemaVersion: 2, RecordedAt: time.Now().Format(time.RFC3339),
		BaseURL: target.String(), GoVersion: runtime.Version(), TestKind: testKind,
		Stage: stage, Report: report,
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if output != "" {
		if err := os.WriteFile(output, payload, 0o644); err != nil {
			return err
		}
	}
	_, _ = os.Stdout.Write(payload)
	if enforceSLO && !report.SLO.Passed {
		return fmt.Errorf("scenario SLO failed: %s", strings.Join(report.SLO.Violations, ","))
	}
	return nil
}

func envOr(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
