package cache

import "github.com/prometheus/client_golang/prometheus"

var (
	requestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_cache_requests_total",
		Help: "Gateway cache lookups by layer and bounded result.",
	}, []string{"layer", "result"})
	refreshTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_cache_refresh_total",
		Help: "Gateway stale-while-revalidate refresh outcomes.",
	}, []string{"result"})
	invalidationTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashmall_cache_invalidation_total",
		Help: "Gateway cache invalidation broadcasts.",
	})
)

func init() {
	prometheus.MustRegister(requestTotal, refreshTotal, invalidationTotal)
}

func recordLookup(layer, result string) {
	requestTotal.WithLabelValues(layer, result).Inc()
}
