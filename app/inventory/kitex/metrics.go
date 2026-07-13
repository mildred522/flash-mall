package main

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type inventoryMetrics struct {
	commandTotal        *prometheus.CounterVec
	commandLatency      *prometheus.HistogramVec
	inconsistentStocks  prometheus.Counter
	pendingReservations prometheus.Gauge
	pending             sync.Map
}

func newInventoryMetrics(registerer prometheus.Registerer) *inventoryMetrics {
	m := &inventoryMetrics{
		commandTotal:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "inventory_kitex_commands_total", Help: "Inventory Kitex command outcomes."}, []string{"operation", "result"}),
		commandLatency:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "inventory_kitex_command_duration_seconds", Help: "Inventory Kitex command latency.", Buckets: prometheus.DefBuckets}, []string{"operation"}),
		inconsistentStocks:  prometheus.NewCounter(prometheus.CounterOpts{Name: "inventory_stock_reconcile_changed_total", Help: "Inventory reconciliation results that found a mismatch."}),
		pendingReservations: prometheus.NewGauge(prometheus.GaugeOpts{Name: "inventory_observed_pending_reservations", Help: "Reservations observed by this process that have not been confirmed or released."}),
	}
	registerer.MustRegister(m.commandTotal, m.commandLatency, m.inconsistentStocks, m.pendingReservations)
	return m
}

func (m *inventoryMetrics) observe(operation string, call func() error) error {
	started := time.Now()
	err := call()
	result := "success"
	if err != nil {
		result = "error"
	}
	m.commandTotal.WithLabelValues(operation, result).Inc()
	m.commandLatency.WithLabelValues(operation).Observe(time.Since(started).Seconds())
	return err
}

func (m *inventoryMetrics) reservationStarted(orderID string) {
	if orderID == "" {
		return
	}
	m.pending.Store(orderID, struct{}{})
	m.refreshPendingReservations()
}

func (m *inventoryMetrics) reservationFinished(orderID string) {
	if orderID == "" {
		return
	}
	m.pending.Delete(orderID)
	m.refreshPendingReservations()
}

func (m *inventoryMetrics) refreshPendingReservations() {
	var count int
	m.pending.Range(func(_, _ any) bool { count++; return true })
	m.pendingReservations.Set(float64(count))
}
