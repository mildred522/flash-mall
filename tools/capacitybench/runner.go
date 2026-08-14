package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type scenarioExecutor interface {
	execute(context.Context, string, int64) sample
}

type loadConfig struct {
	Scenario    string
	Requests    int
	Duration    time.Duration
	RPS         int
	Concurrency int
}

func runLoad(ctx context.Context, executor scenarioExecutor, config loadConfig) scenarioReport {
	if config.Concurrency <= 0 {
		config.Concurrency = 1
	}
	queueSize := config.Concurrency * 2
	if config.RPS > queueSize {
		// Absorb short host/container scheduling pauses without turning timer jitter
		// into a false server-side capacity failure. Sustained backlog still lowers
		// achieved QPS because worker drain time remains part of the measurement.
		queueSize = config.RPS
	}
	jobs := make(chan int64, queueSize)
	var samples []sample
	var samplesMu sync.Mutex
	var workers sync.WaitGroup
	for range config.Concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for sequence := range jobs {
				result := executor.execute(ctx, config.Scenario, sequence)
				samplesMu.Lock()
				samples = append(samples, result)
				samplesMu.Unlock()
			}
		}()
	}

	startedAt := time.Now()
	var sequence atomic.Int64
	var dropped atomic.Int64
	if config.Requests > 0 {
		for range config.Requests {
			jobs <- sequence.Add(1) - 1
			if config.RPS > 0 {
				time.Sleep(time.Second / time.Duration(config.RPS))
			}
		}
	} else {
		duration := config.Duration
		if duration <= 0 {
			duration = 10 * time.Second
		}
		deadline := time.NewTimer(duration)
		if config.RPS > 0 {
			const pacingResolution = time.Millisecond
			ticker := time.NewTicker(pacingResolution)
			defer ticker.Stop()
			measurementStarted := time.Now()
			var scheduled int64
			dispatch := func(elapsed time.Duration) {
				expected := int64(float64(config.RPS) * elapsed.Seconds())
				maximum := int64(float64(config.RPS) * duration.Seconds())
				if expected > maximum {
					expected = maximum
				}
				for scheduled < expected {
					scheduled++
					select {
					case jobs <- sequence.Add(1) - 1:
					default:
						dropped.Add(1)
					}
				}
			}
		scheduleOpen:
			for {
				select {
				case <-ctx.Done():
					break scheduleOpen
				case <-deadline.C:
					dispatch(duration)
					break scheduleOpen
				case <-ticker.C:
					dispatch(time.Since(measurementStarted))
				}
			}
		} else {
		scheduleClosed:
			for {
				select {
				case <-ctx.Done():
					break scheduleClosed
				case <-deadline.C:
					break scheduleClosed
				case jobs <- sequence.Add(1) - 1:
				}
			}
		}
		if !deadline.Stop() {
			select {
			case <-deadline.C:
			default:
			}
		}
	}
	close(jobs)
	workers.Wait()

	report := summarizeSamples(config.Scenario, samples, time.Since(startedAt), int(dropped.Load()))
	report.RPS = config.RPS
	report.Concurrency = config.Concurrency
	report.SLO = evaluateSLO(report, defaultSLO(config.Scenario))
	return report
}

func validateTarget(target *url.URL, scenario string, allowMutation bool, allowCompose bool) error {
	if target == nil || target.Scheme == "" || target.Hostname() == "" {
		return fmt.Errorf("base URL must include scheme and host")
	}
	if scenario == "read" {
		return nil
	}
	if !allowMutation {
		return fmt.Errorf("scenario %s requires -allow-mutation", scenario)
	}
	host := strings.TrimSpace(target.Hostname())
	if allowCompose && target.Scheme == "http" && host == "hertz-gateway" && target.Port() == "8889" {
		return nil
	}
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("mutating scenarios only allow loopback targets")
		}
	}
	return nil
}
