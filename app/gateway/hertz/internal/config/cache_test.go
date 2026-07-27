package config

import (
	"testing"
	"time"
)

func TestCacheConfigAppliesSafeDefaults(t *testing.T) {
	got := (Config{CacheEnableL1: true, CacheEnableL2: true}).CacheConfig()
	if got.L1TTL != 2*time.Second || got.SoftTTL != 30*time.Second || got.HardTTL != 2*time.Minute || got.MaxEntries != 512 {
		t.Fatalf("cache defaults=%+v", got)
	}
}

func TestProductExistenceFilterConfigAppliesSafeDefaults(t *testing.T) {
	got := (Config{}).ProductExistenceConfig()
	if got.Prefix != "flashmall:hertz:existence:product" ||
		got.NegativePrefix != "flashmall:hertz:negative" ||
		got.ExpectedItems != 1_000_000 ||
		got.FalsePositiveRate != 0.01 ||
		got.BatchSize != 1000 ||
		got.NegativeTTL != 30*time.Second {
		t.Fatalf("existence filter defaults=%+v", got)
	}
}
