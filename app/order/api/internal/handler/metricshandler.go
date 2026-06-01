package handler

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func MetricsHandler() http.HandlerFunc {
	return promhttp.Handler().ServeHTTP
}
