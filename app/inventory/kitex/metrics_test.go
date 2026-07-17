package main

import (
	"testing"

	"flash-mall/app/inventory/domain"

	"github.com/prometheus/client_golang/prometheus"
)

func TestReservationMetricsReflectSharedStats(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newInventoryMetrics(registry)
	metrics.refreshReservationStats(domain.ReservationStats{Active: 4, Expired: 2, Processing: 1, DeadLetter: 3})

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, family := range families {
		if family.GetName() != "inventory_reservations" {
			continue
		}
		for _, metric := range family.GetMetric() {
			state := ""
			for _, label := range metric.GetLabel() {
				if label.GetName() == "state" {
					state = label.GetValue()
				}
			}
			values[state] = metric.GetGauge().GetValue()
		}
	}
	if values["active"] != 4 || values["expired"] != 2 || values["processing"] != 1 || values["dead_letter"] != 3 {
		t.Fatalf("reservation metrics=%v", values)
	}
}
