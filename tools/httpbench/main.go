package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	name := flag.String("name", "http", "benchmark name")
	url := flag.String("url", "", "HTTP GET target")
	requests := flag.Int("n", 2000, "measured request count")
	concurrency := flag.Int("c", 20, "concurrent workers")
	warmup := flag.Int("warmup", 200, "warmup request count")
	timeout := flag.Duration("timeout", 5*time.Second, "per-request timeout")
	output := flag.String("out", "", "optional JSON output path")
	flag.Parse()

	config := benchmarkConfig{
		Name:        *name,
		URL:         *url,
		Requests:    *requests,
		Concurrency: *concurrency,
		Timeout:     *timeout,
	}
	if *warmup > 0 {
		warmupConfig := config
		warmupConfig.Name += "-warmup"
		warmupConfig.Requests = *warmup
		if _, err := runBenchmark(context.Background(), warmupConfig); err != nil {
			fatal(err)
		}
	}

	report, err := runBenchmark(context.Background(), config)
	if err != nil {
		fatal(err)
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	payload = append(payload, '\n')
	if *output != "" {
		if err := os.WriteFile(*output, payload, 0o644); err != nil {
			fatal(err)
		}
	}
	_, _ = os.Stdout.Write(payload)
	if report.SuccessRate != 1 {
		os.Exit(1)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
