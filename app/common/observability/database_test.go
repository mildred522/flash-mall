package observability

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
)

func TestConfigureDatabasePoolNormalizesBudget(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cfg := ConfigureDatabasePool(db, DatabasePoolConfig{MaxOpenConns: 8, MaxIdleConns: 20})
	if cfg.MaxOpenConns != 8 || cfg.MaxIdleConns != 8 {
		t.Fatalf("normalized config=%+v", cfg)
	}
	if stats := db.Stats(); stats.MaxOpenConnections != 8 {
		t.Fatalf("max open=%d", stats.MaxOpenConnections)
	}
}

func TestDatabaseStatsCollectorRegistersAndGathers(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ConfigureDatabasePool(db, DatabasePoolConfig{MaxOpenConns: 4, MaxIdleConns: 2})

	registry := prometheus.NewRegistry()
	if err := RegisterDatabaseStats(registry, "orders", db); err != nil {
		t.Fatal(err)
	}
	otherDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer otherDB.Close()
	ConfigureDatabasePool(otherDB, DatabasePoolConfig{MaxOpenConns: 2, MaxIdleConns: 1})
	if err := RegisterDatabaseStats(registry, "products", otherDB); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 4 {
		t.Fatalf("metric families=%d want=4", len(families))
	}
	for _, family := range families {
		if got := len(family.Metric); got < 2 {
			t.Fatalf("metric family %s contains %d pool series", family.GetName(), got)
		}
	}
}
