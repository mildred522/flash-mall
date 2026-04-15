package sessionstate

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestRedisStateStoreSetUserVersion_FailsFastWhenRedisUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	store := NewRedisStateStore(redis.New(addr))

	start := time.Now()
	err = store.SetUserVersion(context.Background(), 1001, 2)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected redis write to fail when redis is unavailable")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected redis failure to return quickly, took %s against %s", elapsed, fmt.Sprintf("%q", addr))
	}
}
