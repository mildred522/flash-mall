package metrics

import "github.com/prometheus/client_golang/prometheus"

// CHG 2026-02-07: 变更=新增业务指标; 之前=仅系统级 metrics; 原因=统计成功率/补偿率/滞留量。
var (
	OrderCreateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "create_total",
			Help:      "Total number of create order requests by result.",
		},
		[]string{"result"},
	)

	OrderSagaSubmitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "saga_submit_total",
			Help:      "Total number of saga submit attempts by result.",
		},
		[]string{"result"},
	)

	OrderCompensationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "compensation_total",
			Help:      "Total number of compensations by type.",
		},
		[]string{"type"},
	)

	DelayQueueBacklog = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "delay_queue_backlog",
			Help:      "Current backlog size for delay queue + processing queue.",
		},
	)

	OrderEventConsumeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "event_consume_total",
			Help:      "Total number of consumed order events by result.",
		},
		[]string{"result"},
	)

	CatalogRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "catalog",
			Name:      "request_total",
			Help:      "Total number of catalog requests by result.",
		},
		[]string{"result"},
	)

	CatalogRequestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "flashmall",
			Subsystem: "catalog",
			Name:      "request_duration_seconds",
			Help:      "Catalog request duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
	)

	PaymentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "payment_total",
			Help:      "Total number of payment attempts by result.",
		},
		[]string{"result"},
	)

	OrderStatusTransitionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "order",
			Name:      "status_transition_total",
			Help:      "Total number of order status transitions.",
		},
		[]string{"from_status", "to_status"},
	)

	CatalogCacheHitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "flashmall",
			Subsystem: "catalog",
			Name:      "cache_hit_total",
			Help:      "Total number of catalog cache lookups by result.",
		},
		[]string{"result"},
	)
)

func init() {
	prometheus.MustRegister(
		OrderCreateTotal, OrderSagaSubmitTotal, OrderCompensationTotal,
		DelayQueueBacklog, OrderEventConsumeTotal,
		CatalogRequestTotal, CatalogRequestDuration, PaymentTotal,
		OrderStatusTransitionTotal, CatalogCacheHitTotal,
	)
}
