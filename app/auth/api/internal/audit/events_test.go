package audit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestMemoryRecorder_ListRecentNewestFirst(t *testing.T) {
	recorder := NewMemoryRecorder(8)
	_ = recorder.Record(context.Background(), Event{EventType: "login_password_fail", Result: "fail"})
	_ = recorder.Record(context.Background(), Event{EventType: "login_password_success", Result: "success"})
	items, err := recorder.ListRecent(context.Background(), 2)
	if err != nil || len(items) != 2 || items[0].EventType != "login_password_success" {
		t.Fatalf("unexpected recent items: %#v err=%v", items, err)
	}
}

func TestRedisRecorder_ListRecentNewestFirst(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	recorder := NewRedisRecorder(redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	}), 8)

	_ = recorder.Record(context.Background(), Event{EventType: "login_password_fail", Result: "fail"})
	_ = recorder.Record(context.Background(), Event{EventType: "login_password_success", Result: "success"})
	items, err := recorder.ListRecent(context.Background(), 2)
	if err != nil || len(items) != 2 || items[0].EventType != "login_password_success" {
		t.Fatalf("unexpected recent items: %#v err=%v", items, err)
	}
	if got := mr.Exists("auth:audit:events"); !got {
		t.Fatalf("expected redis audit list to exist")
	}
}
