package repository

import "testing"

func TestSplitStockAcrossShards(t *testing.T) {
	got := SplitStockAcrossShards(10, 4)
	want := []int64{4, 2, 2, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestStockShardStartIndex(t *testing.T) {
	idx := StockShardStartIndex("order-1", 4)
	if idx < 0 || idx >= 4 {
		t.Fatalf("invalid shard index %d", idx)
	}
}
