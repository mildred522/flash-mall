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
)

func init() {
	prometheus.MustRegister(OrderCreateTotal, OrderSagaSubmitTotal, OrderCompensationTotal, DelayQueueBacklog, OrderEventConsumeTotal)
}
