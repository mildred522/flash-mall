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
