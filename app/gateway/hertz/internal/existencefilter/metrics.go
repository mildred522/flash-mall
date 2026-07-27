package existencefilter

import "github.com/prometheus/client_golang/prometheus"

var (
	filterChecks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_existence_filter_checks_total",
		Help: "Product existence filter checks by bounded result.",
	}, []string{"entity", "result"})
	filterUpdates = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_existence_filter_updates_total",
		Help: "Product existence filter updates by bounded result.",
	}, []string{"entity", "result"})
	filterRebuilds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_existence_filter_rebuild_total",
		Help: "Product existence filter rebuilds by bounded result.",
	}, []string{"entity", "result"})
	filterRebuildDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "flashmall_existence_filter_rebuild_duration_seconds",
		Help: "Product existence filter rebuild duration.",
	}, []string{"entity"})
	filterItems = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "flashmall_existence_filter_items",
		Help: "Items loaded into the active product existence filter.",
	}, []string{"entity"})
	filterReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "flashmall_existence_filter_ready",
		Help: "Whether the product existence filter is ready.",
	}, []string{"entity"})
	filterFalsePositives = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_existence_filter_false_positive_total",
		Help: "Positive filter checks disproved by the authoritative product query.",
	}, []string{"entity"})
	negativeRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_negative_cache_requests_total",
		Help: "Product negative cache lookups by bounded result.",
	}, []string{"entity", "result"})
)

func init() {
	prometheus.MustRegister(
		filterChecks,
		filterUpdates,
		filterRebuilds,
		filterRebuildDuration,
		filterItems,
		filterReady,
		filterFalsePositives,
		negativeRequests,
	)
}

func recordCheck(result Result, err error) {
	label := string(result)
	if err != nil {
		label = "error"
	}
	if label == "" {
		label = string(ResultUnready)
	}
	filterChecks.WithLabelValues("product", label).Inc()
	if err != nil || result == ResultUnready {
		filterReady.WithLabelValues("product").Set(0)
	} else {
		filterReady.WithLabelValues("product").Set(1)
	}
}

func RecordFalsePositive() {
	filterFalsePositives.WithLabelValues("product").Inc()
}
