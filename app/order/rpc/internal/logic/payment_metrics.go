package logic

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var paymentTransitionResults = map[string]struct{}{
	"success":         {},
	"idempotent":      {},
	"amount_mismatch": {},
	"closed":          {},
	"not_payable":     {},
	"error":           {},
}

var (
	paymentTransitionTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_payment_state_transition_total",
		Help: "Payment state transition outcomes, including idempotent callbacks.",
	}, []string{"result"})
	paymentTransitionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "flashmall_payment_state_transition_duration_seconds",
		Help:    "Payment state transition latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
)

func init() {
	prometheus.MustRegister(paymentTransitionTotal, paymentTransitionDuration)
	for result := range paymentTransitionResults {
		paymentTransitionTotal.WithLabelValues(result).Add(0)
		paymentTransitionDuration.WithLabelValues(result)
	}
}

func normalizePaymentTransitionResult(result string) string {
	result = strings.ToLower(strings.TrimSpace(result))
	if _, ok := paymentTransitionResults[result]; ok {
		return result
	}
	return "error"
}

func recordPaymentTransition(result string, duration time.Duration) {
	result = normalizePaymentTransitionResult(result)
	paymentTransitionTotal.WithLabelValues(result).Inc()
	paymentTransitionDuration.WithLabelValues(result).Observe(duration.Seconds())
}
