package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/prometheus/client_golang/prometheus"
)

type RequestMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

func NewRequestMetrics(registerer prometheus.Registerer) *RequestMetrics {
	metrics := &RequestMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flashmall_http_requests_total",
			Help: "Hertz HTTP request outcomes.",
		}, []string{"method", "route", "status_class"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "flashmall_http_request_duration_seconds",
			Help:    "Hertz HTTP request latency.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		}, []string{"method", "route", "status_class"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flashmall_http_in_flight_requests",
			Help: "Hertz HTTP requests currently being handled.",
		}),
	}
	registerer.MustRegister(metrics.requests, metrics.duration, metrics.inFlight)
	return metrics
}

func (m *RequestMetrics) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		c.Next(ctx)

		method := string(c.Method())
		route := metricRoute(c.FullPath(), string(c.Path()))
		class := statusClass(c.Response.StatusCode())
		m.requests.WithLabelValues(method, route, class).Inc()
		m.duration.WithLabelValues(method, route, class).Observe(time.Since(startedAt).Seconds())
	}
}

func metricRoute(fullPath, requestPath string) string {
	if strings.HasPrefix(requestPath, "/api/") {
		if strings.HasPrefix(fullPath, "/api/") {
			return fullPath
		}
		return "unmatched_api"
	}
	switch fullPath {
	case "/live", "/ready", "/health", "/metrics":
		return fullPath
	default:
		return "static"
	}
}

func statusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

var defaultRequestMetrics = NewRequestMetrics(prometheus.DefaultRegisterer)

func ObserveRequests() app.HandlerFunc {
	return defaultRequestMetrics.Middleware()
}
