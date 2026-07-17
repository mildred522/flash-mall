package job

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var knownOutboxEventTypes = map[string]struct{}{
	"order.created":  {},
	"order.paid":     {},
	"order.refunded": {},
}

var (
	outboxPublishTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_outbox_publish_total",
		Help: "Outbox publisher outcomes by bounded event type and result.",
	}, []string{"event_type", "result"})
	outboxPublishDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "flashmall_outbox_publish_duration_seconds",
		Help:    "Outbox publish latency by bounded event type and result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"event_type", "result"})
	outboxState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "flashmall_outbox_events",
		Help: "Current outbox event counts by state.",
	}, []string{"state"})
)

func init() {
	prometheus.MustRegister(outboxPublishTotal, outboxPublishDuration, outboxState)
	for _, state := range []string{"pending", "publishing", "dead"} {
		outboxState.WithLabelValues(state).Set(0)
	}
}

func normalizeOutboxEventType(eventType string) string {
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	if _, ok := knownOutboxEventTypes[eventType]; ok {
		return eventType
	}
	return "other"
}

func recordOutboxPublish(eventType, result string, duration time.Duration) {
	eventType = normalizeOutboxEventType(eventType)
	outboxPublishTotal.WithLabelValues(eventType, result).Inc()
	outboxPublishDuration.WithLabelValues(eventType, result).Observe(duration.Seconds())
}
