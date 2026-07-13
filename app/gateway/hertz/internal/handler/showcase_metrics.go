package handler

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var showcaseInvalidReasons = []string{
	"product_not_found",
	"product_inactive",
	"merchant_not_found",
	"merchant_inactive",
	"out_of_stock",
}

var (
	showcaseReadTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_showcase_read_total",
		Help: "Total showcase read requests by audience and result.",
	}, []string{"audience", "result"})
	showcasePublishTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_showcase_publish_total",
		Help: "Total showcase publish requests by bounded result.",
	}, []string{"result"})
	showcaseActiveItems = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flashmall_showcase_active_items",
		Help: "Current number of valid products visible in the homepage showcase.",
	})
	showcasePendingSlots = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flashmall_showcase_pending_slots",
		Help: "Current number of empty or invalid homepage showcase slots.",
	})
	showcaseInvalidSlots = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "flashmall_showcase_invalid_slots",
		Help: "Current showcase slots rejected for each bounded reason.",
	}, []string{"reason"})
	showcaseCandidateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "flashmall_showcase_candidate_duration_seconds",
		Help:    "Showcase candidate request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	})
	showcaseCandidateTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_showcase_candidate_total",
		Help: "Total showcase candidate requests by result.",
	}, []string{"result"})
	showcaseCandidateCacheTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_showcase_candidate_cache_total",
		Help: "Total showcase candidate cache lookups by hit or miss.",
	}, []string{"result"})
	storeRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "flashmall_store_request_duration_seconds",
		Help:    "Public store request duration by operation and result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "result"})
	showcaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "flashmall_showcase_request_duration_seconds",
		Help:    "Showcase read request duration by audience and result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"audience", "result"})
)

func init() {
	prometheus.MustRegister(
		showcaseReadTotal,
		showcasePublishTotal,
		showcaseActiveItems,
		showcasePendingSlots,
		showcaseInvalidSlots,
		showcaseCandidateDuration,
		showcaseCandidateTotal,
		showcaseCandidateCacheTotal,
		storeRequestDuration,
		showcaseRequestDuration,
	)
	for _, reason := range showcaseInvalidReasons {
		showcaseInvalidSlots.WithLabelValues(reason).Set(0)
	}
	for _, audience := range []string{"public", "admin"} {
		for _, result := range []string{"success", "error"} {
			showcaseReadTotal.WithLabelValues(audience, result).Add(0)
			showcaseRequestDuration.WithLabelValues(audience, result)
		}
	}
	for _, result := range []string{"success", "conflict", "invalid", "error"} {
		showcasePublishTotal.WithLabelValues(result).Add(0)
	}
	for _, result := range []string{"success", "error"} {
		showcaseCandidateTotal.WithLabelValues(result).Add(0)
		for _, operation := range []string{"detail", "products"} {
			storeRequestDuration.WithLabelValues(operation, result)
		}
	}
	for _, result := range []string{"hit", "miss"} {
		showcaseCandidateCacheTotal.WithLabelValues(result).Add(0)
	}
}

func recordShowcaseRead(audience, result string, layout ShowcaseResp, duration time.Duration) {
	showcaseReadTotal.WithLabelValues(audience, result).Inc()
	showcaseRequestDuration.WithLabelValues(audience, result).Observe(duration.Seconds())
	if result != "success" {
		return
	}
	active := 0
	invalidCounts := make(map[string]int, len(showcaseInvalidReasons))
	for _, slot := range layout.Items {
		if slot.Valid && !slot.Empty && slot.ProductID > 0 {
			active++
			continue
		}
		if slot.InvalidReason != "" {
			invalidCounts[slot.InvalidReason]++
		}
	}
	showcaseActiveItems.Set(float64(active))
	pending := 12 - active
	if pending < 0 {
		pending = 0
	}
	showcasePendingSlots.Set(float64(pending))
	for _, reason := range showcaseInvalidReasons {
		showcaseInvalidSlots.WithLabelValues(reason).Set(float64(invalidCounts[reason]))
	}
}

func recordShowcasePublish(result string) {
	showcasePublishTotal.WithLabelValues(result).Inc()
}

func recordShowcaseCandidate(result string, duration time.Duration) {
	showcaseCandidateTotal.WithLabelValues(result).Inc()
	showcaseCandidateDuration.Observe(duration.Seconds())
}

func recordStoreRequest(operation, result string, duration time.Duration) {
	storeRequestDuration.WithLabelValues(operation, result).Observe(duration.Seconds())
}
