package handler

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var paymentCallbackResults = map[string]struct{}{
	"success":         {},
	"invalid_request": {},
	"unauthorized":    {},
	"invalid_payload": {},
	"error":           {},
}

var (
	paymentCallbackTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashmall_payment_callback_total",
		Help: "Payment callback outcomes with bounded result labels.",
	}, []string{"result"})
	paymentCallbackDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "flashmall_payment_callback_duration_seconds",
		Help:    "Payment callback processing latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
)

func init() {
	prometheus.MustRegister(paymentCallbackTotal, paymentCallbackDuration)
	for result := range paymentCallbackResults {
		paymentCallbackTotal.WithLabelValues(result).Add(0)
		paymentCallbackDuration.WithLabelValues(result)
	}
}

func recordPaymentCallback(result string, duration time.Duration) {
	if _, ok := paymentCallbackResults[result]; !ok {
		result = "error"
	}
	paymentCallbackTotal.WithLabelValues(result).Inc()
	paymentCallbackDuration.WithLabelValues(result).Observe(duration.Seconds())
}
