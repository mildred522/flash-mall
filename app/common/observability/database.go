package observability

import (
	"database/sql"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type DatabasePoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int64
	ConnMaxIdleTimeSeconds int64
}

func (c DatabasePoolConfig) normalized() DatabasePoolConfig {
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = 16
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = c.MaxOpenConns / 2
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		c.MaxIdleConns = c.MaxOpenConns
	}
	if c.ConnMaxLifetimeSeconds <= 0 {
		c.ConnMaxLifetimeSeconds = 300
	}
	if c.ConnMaxIdleTimeSeconds <= 0 {
		c.ConnMaxIdleTimeSeconds = 120
	}
	return c
}

func ConfigureDatabasePool(db *sql.DB, cfg DatabasePoolConfig) DatabasePoolConfig {
	cfg = cfg.normalized()
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeSeconds) * time.Second)
	return cfg
}

type databaseStatsCollector struct {
	db           *sql.DB
	connections  *prometheus.Desc
	waitTotal    *prometheus.Desc
	waitDuration *prometheus.Desc
	closedTotal  *prometheus.Desc
}

func RegisterDatabaseStats(registerer prometheus.Registerer, pool string, db *sql.DB) error {
	labels := prometheus.Labels{"pool": pool}
	err := registerer.Register(&databaseStatsCollector{
		db: db,
		connections: prometheus.NewDesc(
			"flashmall_db_connections", "Database pool connections by state.", []string{"state"}, labels,
		),
		waitTotal: prometheus.NewDesc(
			"flashmall_db_wait_total", "Database pool waits for a free connection.", nil, labels,
		),
		waitDuration: prometheus.NewDesc(
			"flashmall_db_wait_duration_seconds_total", "Total time waiting for a database connection.", nil, labels,
		),
		closedTotal: prometheus.NewDesc(
			"flashmall_db_connections_closed_total", "Database connections closed by pool policy.", []string{"reason"}, labels,
		),
	})
	var alreadyRegistered prometheus.AlreadyRegisteredError
	if errors.As(err, &alreadyRegistered) {
		return nil
	}
	return err
}

func (c *databaseStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.connections
	ch <- c.waitTotal
	ch <- c.waitDuration
	ch <- c.closedTotal
}

func (c *databaseStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	for state, value := range map[string]int{
		"max_open": stats.MaxOpenConnections,
		"open":     stats.OpenConnections,
		"in_use":   stats.InUse,
		"idle":     stats.Idle,
	} {
		ch <- prometheus.MustNewConstMetric(c.connections, prometheus.GaugeValue, float64(value), state)
	}
	ch <- prometheus.MustNewConstMetric(c.waitTotal, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, stats.WaitDuration.Seconds())
	for reason, value := range map[string]int64{
		"max_idle":  stats.MaxIdleClosed,
		"idle_time": stats.MaxIdleTimeClosed,
		"lifetime":  stats.MaxLifetimeClosed,
	} {
		ch <- prometheus.MustNewConstMetric(c.closedTotal, prometheus.CounterValue, float64(value), reason)
	}
}
