package assetstore

import (
	"context"
	"sync"
	"time"
)

type IntegrityAudit interface {
	Check(context.Context) (IntegrityReport, error)
}

type IntegrityProbe interface {
	Probe(context.Context) (IntegrityReport, error)
}

type IntegritySnapshot struct {
	Report    IntegrityReport `json:"report"`
	CheckedAt time.Time       `json:"checked_at,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func (s IntegritySnapshot) Ready() bool {
	return s.Error == "" && !s.CheckedAt.IsZero() && s.Report.Configured && s.Report.Writable
}

func (s IntegritySnapshot) Degraded() bool {
	return s.Ready() && !s.Report.Healthy()
}

type IntegrityReporter interface {
	Snapshot() IntegritySnapshot
}

type IntegrityMonitor struct {
	audit    IntegrityAudit
	interval time.Duration

	mu       sync.RWMutex
	snapshot IntegritySnapshot
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewIntegrityMonitor(audit IntegrityAudit, interval time.Duration) *IntegrityMonitor {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &IntegrityMonitor{audit: audit, interval: interval}
}

func (m *IntegrityMonitor) RunOnce(ctx context.Context) {
	if m == nil || m.audit == nil {
		return
	}
	report, err := m.audit.Check(ctx)
	m.record(report, err)
}

func (m *IntegrityMonitor) Prime(ctx context.Context) {
	if m == nil || m.audit == nil {
		return
	}
	probe, ok := m.audit.(IntegrityProbe)
	if !ok {
		m.RunOnce(ctx)
		return
	}
	report, err := probe.Probe(ctx)
	m.record(report, err)
}

func (m *IntegrityMonitor) record(report IntegrityReport, err error) {
	snapshot := IntegritySnapshot{Report: report, CheckedAt: time.Now()}
	if err != nil {
		snapshot.Error = err.Error()
	}
	m.mu.Lock()
	m.snapshot = snapshot
	m.mu.Unlock()
	observeIntegritySnapshot(snapshot)
}

func (m *IntegrityMonitor) Start(parent context.Context) {
	if m == nil || m.audit == nil {
		return
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.done = make(chan struct{})
	done := m.done
	m.mu.Unlock()

	go func() {
		defer close(done)
		m.RunOnce(ctx)
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.RunOnce(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *IntegrityMonitor) Snapshot() IntegritySnapshot {
	if m == nil {
		return IntegritySnapshot{Error: "upload integrity monitor is not configured"}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot
}

func (m *IntegrityMonitor) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	cancel := m.cancel
	done := m.done
	m.cancel = nil
	m.done = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}
