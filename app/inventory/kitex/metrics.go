package main

import (
	"time"

	"flash-mall/app/inventory/domain"

	"github.com/prometheus/client_golang/prometheus"
)

type inventoryMetrics struct {
	commandTotal        *prometheus.CounterVec
	commandLatency      *prometheus.HistogramVec
	inconsistentStocks  prometheus.Counter
	reservationStates   *prometheus.GaugeVec
	reservationRecovery *prometheus.CounterVec
}

func newInventoryMetrics(registerer prometheus.Registerer) *inventoryMetrics {
	m := &inventoryMetrics{
		commandTotal:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "inventory_kitex_commands_total", Help: "Inventory Kitex command outcomes."}, []string{"operation", "result"}),
		commandLatency:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "inventory_kitex_command_duration_seconds", Help: "Inventory Kitex command latency.", Buckets: prometheus.DefBuckets}, []string{"operation"}),
		inconsistentStocks:  prometheus.NewCounter(prometheus.CounterOpts{Name: "inventory_stock_reconcile_changed_total", Help: "Inventory reconciliation results that found a mismatch."}),
		reservationStates:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "inventory_reservations", Help: "Shared Redis reservation lifecycle counts."}, []string{"state"}),
		reservationRecovery: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "inventory_reservation_recovery_total", Help: "Expired reservation recovery outcomes."}, []string{"result"}),
	}
	registerer.MustRegister(m.commandTotal, m.commandLatency, m.inconsistentStocks, m.reservationStates, m.reservationRecovery)
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

func (m *inventoryMetrics) refreshReservationStats(stats domain.ReservationStats) {
	m.reservationStates.WithLabelValues("active").Set(float64(stats.Active))
	m.reservationStates.WithLabelValues("expired").Set(float64(stats.Expired))
	m.reservationStates.WithLabelValues("processing").Set(float64(stats.Processing))
	m.reservationStates.WithLabelValues("dead_letter").Set(float64(stats.DeadLetter))
}
