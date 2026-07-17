package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
)

func TestCoordinatorL1AvoidsDuplicateLoads(t *testing.T) {
	coordinator := New(Config{EnableL1: true, L1TTL: time.Second, MaxEntries: 16}, nil)
	loads := 0
	loader := func(context.Context) ([]byte, error) {
		loads++
		return []byte(`{"value":1}`), nil
	}
	first, source, err := coordinator.GetOrLoad(context.Background(), "homepage", loader)
	if err != nil || source != SourceOrigin || string(first) != `{"value":1}` {
		t.Fatalf("first value=%s source=%s err=%v", first, source, err)
	}
	second, source, err := coordinator.GetOrLoad(context.Background(), "homepage", loader)
	if err != nil || source != SourceL1 || string(second) != string(first) || loads != 1 {
		t.Fatalf("second value=%s source=%s loads=%d err=%v", second, source, loads, err)
	}
}

func TestCoordinatorSharesL2AcrossReplicas(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	cfg := Config{Prefix: "test", EnableL2: true, SoftTTL: time.Minute, HardTTL: 2 * time.Minute}
	first := New(cfg, client)
	second := New(cfg, client)
	loads := 0
	loader := func(context.Context) ([]byte, error) {
		loads++
		return []byte(`{"value":2}`), nil
	}
	if _, source, err := first.GetOrLoad(context.Background(), "store:1", loader); err != nil || source != SourceOrigin {
		t.Fatalf("first source=%s err=%v", source, err)
	}
	value, source, err := second.GetOrLoad(context.Background(), "store:1", loader)
	if err != nil || source != SourceL2 || string(value) != `{"value":2}` || loads != 1 {
		t.Fatalf("second value=%s source=%s loads=%d err=%v", value, source, loads, err)
	}
}

func TestCoordinatorFallsBackToOriginWhenL2WriteFails(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	coordinator := New(Config{Prefix: "unavailable", EnableL2: true, SoftTTL: time.Minute, HardTTL: 2 * time.Minute, OperationTimeout: 100 * time.Millisecond}, client)
	mr.Close()

	started := time.Now()
	value, source, err := coordinator.GetOrLoad(context.Background(), "homepage", func(context.Context) ([]byte, error) {
		return []byte(`{"value":"origin"}`), nil
	})
	if err != nil || source != SourceOrigin || string(value) != `{"value":"origin"}` {
		t.Fatalf("value=%s source=%s err=%v, want successful origin fallback", value, source, err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("origin fallback took %s, want fail-fast cache bypass", elapsed)
	}
}

func TestCoordinatorInvalidationReachesPeerL1(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	cfg := Config{Prefix: "invalidation", EnableL1: true, EnableL2: true, L1TTL: time.Minute, SoftTTL: time.Minute, HardTTL: 2 * time.Minute}
	first := New(cfg, client)
	second := New(cfg, client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first.Start(ctx)
	second.Start(ctx)
	defer first.Close()
	defer second.Close()

	value := []byte(`{"version":1}`)
	loader := func(context.Context) ([]byte, error) { return value, nil }
	if _, _, err := first.GetOrLoad(ctx, "homepage", loader); err != nil {
		t.Fatal(err)
	}
	if _, _, err := second.GetOrLoad(ctx, "homepage", loader); err != nil {
		t.Fatal(err)
	}
	value = []byte(`{"version":2}`)
	if err := first.Invalidate(ctx, "homepage"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		got, source, err := second.GetOrLoad(ctx, "homepage", loader)
		if err == nil && source == SourceOrigin && string(got) == string(value) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("peer cache was not invalidated: value=%s source=%s err=%v", got, source, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCoordinatorInvalidatesPrefix(t *testing.T) {
	coordinator := New(Config{EnableL1: true, L1TTL: time.Minute}, nil)
	value := []byte(`{"version":1}`)
	loader := func(context.Context) ([]byte, error) { return value, nil }
	for _, key := range []string{"store:products:7:1", "store:products:7:2"} {
		if _, _, err := coordinator.GetOrLoad(context.Background(), key, loader); err != nil {
			t.Fatal(err)
		}
	}
	coordinator.InvalidatePrefix(context.Background(), "store:products:7:")
	value = []byte(`{"version":2}`)
	for _, key := range []string{"store:products:7:1", "store:products:7:2"} {
		got, source, err := coordinator.GetOrLoad(context.Background(), key, loader)
		if err != nil || source != SourceOrigin || string(got) != string(value) {
			t.Fatalf("key=%s value=%s source=%s err=%v", key, got, source, err)
		}
	}
}
