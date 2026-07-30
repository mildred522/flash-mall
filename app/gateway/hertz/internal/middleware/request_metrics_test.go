package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRequestMetricsRecordsBoundedAPILabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewRequestMetrics(registry)
	h := server.Default()
	h.Use(metrics.Middleware())
	h.GET("/api/shop/products/detail", func(_ context.Context, c *app.RequestContext) {
		c.Status(consts.StatusNotFound)
	})

	response := ut.PerformRequest(h.Engine, "GET", "/api/shop/products/detail?product_id=999", nil).Result()
	if response.StatusCode() != consts.StatusNotFound {
		t.Fatalf("status=%d", response.StatusCode())
	}

	if got := testutil.ToFloat64(metrics.requests.WithLabelValues(
		"GET", "/api/shop/products/detail", "4xx",
	)); got != 1 {
		t.Fatalf("request total=%v want=1", got)
	}
	if got := testutil.ToFloat64(metrics.inFlight); got != 0 {
		t.Fatalf("in-flight requests=%v want=0", got)
	}
}

func TestRequestMetricsCollapsesStaticPaths(t *testing.T) {
	for _, path := range []string{
		"/assets/catalog-a1b2c3.png",
		"/assets/catalog-d4e5f6.png",
		"/store/1101",
	} {
		if got := metricRoute("", path); got != "static" {
			t.Fatalf("metricRoute(%q)=%q want=static", path, got)
		}
	}

	for _, path := range []string{"/live", "/ready", "/health", "/metrics"} {
		if got := metricRoute(path, path); got != path {
			t.Fatalf("metricRoute(%q)=%q want=%q", path, got, path)
		}
	}
}

func TestMetricRouteUsesMatchedTemplateAndCollapsesUnknownAPIs(t *testing.T) {
	if got := metricRoute("/api/shop/products/:id", "/api/shop/products/42"); got != "/api/shop/products/:id" {
		t.Fatalf("matched route=%q", got)
	}
	for _, path := range []string{"/api/not-found/one", "/api/not-found/two"} {
		if got := metricRoute("", path); got != "unmatched_api" {
			t.Fatalf("metricRoute(%q)=%q want=unmatched_api", path, got)
		}
	}
}

func TestStatusClassHasFixedCardinality(t *testing.T) {
	for status, want := range map[int]string{
		0:   "unknown",
		200: "2xx",
		302: "3xx",
		404: "4xx",
		503: "5xx",
		700: "unknown",
	} {
		if got := statusClass(status); got != want {
			t.Fatalf("statusClass(%d)=%q want=%q", status, got, want)
		}
	}
}
