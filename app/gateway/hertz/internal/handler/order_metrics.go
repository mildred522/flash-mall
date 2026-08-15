package handler

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	orderStageGID        = "dtm_new_gid"
	orderStageSagaSubmit = "dtm_saga_submit"
	orderStageQuery      = "order_result_query"
)

var (
	orderStageDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "flashmall_order_create_stage_duration_seconds",
		Help: "Create-order latency split by bounded orchestration stage and result.",
		Buckets: []float64{
			0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5,
		},
	}, []string{"stage", "result"})
	orderStageTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_order_create_stage_total",
		Help: "Create-order stage outcomes with bounded labels.",
	}, []string{"stage", "result"})
)

func init() {
	prometheus.MustRegister(orderStageDuration, orderStageTotal)
	for _, stage := range []string{orderStageGID, orderStageSagaSubmit, orderStageQuery} {
		for _, result := range []string{"success", "error"} {
			orderStageDuration.WithLabelValues(stage, result)
			orderStageTotal.WithLabelValues(stage, result).Add(0)
		}
	}
}

func recordOrderStage(stage string, started time.Time, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	orderStageDuration.WithLabelValues(stage, result).Observe(time.Since(started).Seconds())
	orderStageTotal.WithLabelValues(stage, result).Inc()
}
