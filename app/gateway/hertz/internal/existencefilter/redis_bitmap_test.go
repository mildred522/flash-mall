package existencefilter

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	redis "github.com/redis/go-redis/v9"
)

type sliceIDSource struct {
	ids []int64
}

type failingIDSource struct {
	err error
}

func (s failingIDSource) ProductIDBatch(context.Context, int64, int) ([]int64, error) {
	return nil, s.err
}

func (s sliceIDSource) ProductIDBatch(_ context.Context, afterID int64, limit int) ([]int64, error) {
	result := make([]int64, 0, limit)
	for _, id := range s.ids {
		if id > afterID {
			result = append(result, id)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

func TestBitPositionsStayCompatibleWithPublishedAlgorithm(t *testing.T) {
	got := bitPositions(100, 9_585_059, 7)
	want := []int64{5_467_697, 7_528_761, 4_766, 885_623, 2_946_687, 5_007_751, 5_888_608}
	if hashAlgorithm != "xxhash64-double-v1" || !reflect.DeepEqual(got, want) {
		t.Fatalf("algorithm=%s positions=%v", hashAlgorithm, got)
	}
}

func TestRedisBitmapRebuildPublishesOnlyCompleteGeneration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	filter := NewRedisBitmap(Config{
		Prefix: "test:products", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 2, OperationTimeout: time.Second,
	}, client, sliceIDSource{ids: []int64{101, 102, 205}})

	result, err := filter.Check(context.Background(), 101)
	if err != nil || result != ResultUnready {
		t.Fatalf("before rebuild result=%s err=%v", result, err)
	}
	if err := filter.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{101, 102, 205} {
		result, err = filter.Check(context.Background(), id)
		if err != nil || result != ResultPossible {
			t.Fatalf("id=%d result=%s err=%v", id, result, err)
		}
	}
	result, err = filter.Check(context.Background(), 999999)
	if err != nil || result != ResultAbsent {
		t.Fatalf("missing result=%s err=%v", result, err)
	}
	status := filter.Status(context.Background())
	if !status.Ready || status.Items != 3 || status.Generation == "" {
		t.Fatalf("status=%+v", status)
	}
}

func TestRedisBitmapRebuildFailureKeepsPreviousGeneration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	filter := NewRedisBitmap(Config{
		Prefix: "test:failure", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}, client, sliceIDSource{ids: []int64{301}})
	if err := filter.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	activeBefore, err := mr.Get("test:failure:active")
	if err != nil {
		t.Fatal(err)
	}
	filter.source = failingIDSource{err: errors.New("mysql unavailable")}
	if err := filter.Rebuild(context.Background()); err == nil {
		t.Fatal("rebuild should fail")
	}
	activeAfter, err := mr.Get("test:failure:active")
	if err != nil {
		t.Fatal(err)
	}
	if activeAfter != activeBefore {
		t.Fatalf("active generation changed from %q to %q", activeBefore, activeAfter)
	}
	result, err := filter.Check(context.Background(), 301)
	if err != nil || result != ResultPossible {
		t.Fatalf("previous generation unusable: result=%s err=%v", result, err)
	}
}

func TestRedisBitmapInstancesShareActiveGeneration(t *testing.T) {
	mr := miniredis.RunT(t)
	clientA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})
	config := Config{
		Prefix: "test:shared", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}
	writer := NewRedisBitmap(config, clientA, sliceIDSource{ids: []int64{401}})
	reader := NewRedisBitmap(config, clientB, nil)
	if err := writer.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := reader.Check(context.Background(), 401)
	if err != nil || result != ResultPossible {
		t.Fatalf("shared result=%s err=%v", result, err)
	}
	if writer.Status(context.Background()).Generation != reader.Status(context.Background()).Generation {
		t.Fatal("instances should observe the same active generation")
	}
}

func TestRedisBitmapUnavailableNeverReturnsAbsent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	filter := NewRedisBitmap(Config{
		Prefix: "test:unavailable", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: 50 * time.Millisecond,
	}, client, sliceIDSource{ids: []int64{501}})
	if err := filter.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	mr.Close()
	result, err := filter.Check(context.Background(), 501)
	if err == nil || result == ResultAbsent {
		t.Fatalf("result=%s err=%v; unavailable Redis must fail open", result, err)
	}
	_ = client.Close()
}

func TestRedisBitmapAddIsIdempotent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	filter := NewRedisBitmap(Config{
		Prefix: "test:add", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}, client, sliceIDSource{})
	if err := filter.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := filter.Add(context.Background(), 777); err != nil {
		t.Fatal(err)
	}
	if err := filter.Add(context.Background(), 777); err != nil {
		t.Fatal(err)
	}
	result, err := filter.Check(context.Background(), 777)
	if err != nil || result != ResultPossible {
		t.Fatalf("result=%s err=%v", result, err)
	}
}

func TestRedisBitmapAddRefusesPublicationDuringRebuild(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	filter := NewRedisBitmap(Config{
		Prefix: "test:add-lock", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}, client, sliceIDSource{})
	if err := filter.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	mr.Set("test:add-lock:rebuild-lock", "another-instance")
	if err := filter.Add(context.Background(), 778); !errors.Is(err, ErrRebuildInProgress) {
		t.Fatalf("err=%v, want ErrRebuildInProgress", err)
	}
}

func TestRedisBitmapStartBuildsMissingFilter(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	filter := NewRedisBitmap(Config{
		Prefix: "test:start", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}, client, sliceIDSource{ids: []int64{501}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	filter.Start(ctx, time.Hour, func(err error) { t.Errorf("maintenance error: %v", err) })

	deadline := time.Now().Add(time.Second)
	for {
		result, err := filter.Check(context.Background(), 501)
		if err == nil && result == ResultPossible {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("filter did not become ready: result=%s err=%v", result, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRedisBitmapStartInitializesReplicaMetricsFromActiveGeneration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	config := Config{
		Prefix: "test:replica-metrics", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}
	writer := NewRedisBitmap(config, client, sliceIDSource{ids: []int64{601, 602}})
	if err := writer.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	filterItems.WithLabelValues("product").Set(0)
	filterReady.WithLabelValues("product").Set(0)
	reader := NewRedisBitmap(config, client, sliceIDSource{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader.Start(ctx, time.Hour, func(err error) { t.Errorf("maintenance error: %v", err) })

	deadline := time.Now().Add(time.Second)
	for {
		if testutil.ToFloat64(filterReady.WithLabelValues("product")) == 1 &&
			testutil.ToFloat64(filterItems.WithLabelValues("product")) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ready=%v items=%v",
				testutil.ToFloat64(filterReady.WithLabelValues("product")),
				testutil.ToFloat64(filterItems.WithLabelValues("product")))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRedisNegativeCacheLifecycle(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewRedisNegativeCache(Config{
		Prefix: "test:negative", NegativeTTL: 30 * time.Second, OperationTimeout: time.Second,
	}, client)

	hit, err := cache.Contains(context.Background(), 404)
	if err != nil || hit {
		t.Fatalf("initial hit=%t err=%v", hit, err)
	}
	if err := cache.Mark(context.Background(), 404); err != nil {
		t.Fatal(err)
	}
	hit, err = cache.Contains(context.Background(), 404)
	if err != nil || !hit {
		t.Fatalf("marked hit=%t err=%v", hit, err)
	}
	if err := cache.Invalidate(context.Background(), 404); err != nil {
		t.Fatal(err)
	}
	hit, err = cache.Contains(context.Background(), 404)
	if err != nil || hit {
		t.Fatalf("invalidated hit=%t err=%v", hit, err)
	}
}

func TestExistenceFilterMetricsAreGathered(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	filter := NewRedisBitmap(Config{
		Prefix: "test:metrics", ExpectedItems: 1000, FalsePositiveRate: 0.001,
		BatchSize: 10, OperationTimeout: time.Second,
	}, client, sliceIDSource{ids: []int64{901}})
	if err := filter.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := filter.Check(context.Background(), 901); err != nil {
		t.Fatal(err)
	}
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	for _, name := range []string{
		"flashmall_existence_filter_checks_total",
		"flashmall_existence_filter_rebuild_total",
		"flashmall_existence_filter_items",
		"flashmall_existence_filter_ready",
	} {
		if !names[name] {
			t.Errorf("metric missing: %s", name)
		}
	}
}
