package handler

import (
	"context"
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/inventoryclient"
	"flash-mall/app/gateway/hertz/internal/svc"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type runtimeStateClientStub struct {
	state inventoryclient.RuntimeState
	err   error
}

func (s runtimeStateClientStub) GetRuntimeState(context.Context, *common.RequestMeta) (inventoryclient.RuntimeState, error) {
	return s.state, s.err
}

func TestHealthRejectsMissingConfiguredInventoryClient(t *testing.T) {
	h := server.Default()
	h.GET("/health", HealthHandler(&svc.ServiceContext{Config: config.Config{
		Name:                   "gateway-test",
		InventoryKitexEndpoint: "inventory-kitex:8093",
	}}, time.Now()))

	resp := ut.PerformRequest(h.Engine, "GET", "/health", nil).Result()
	if resp.StatusCode() != consts.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestInventoryRuntimeReadinessRequiresAuthoritativeDeduct(t *testing.T) {
	_, err := checkInventoryRuntime(context.Background(), runtimeStateClientStub{state: inventoryclient.RuntimeState{
		RedisConfigured: true,
		MySQLConfigured: true,
		ShardCount:      4,
	}})
	if err == nil {
		t.Fatal("readiness must reject inventory runtime with final deduct disabled")
	}
}

func TestInventoryRuntimeReadinessAcceptsAuthoritativeRuntime(t *testing.T) {
	state, err := checkInventoryRuntime(context.Background(), runtimeStateClientStub{state: inventoryclient.RuntimeState{
		FinalDeductEnabled:       true,
		RedisConfigured:          true,
		MySQLConfigured:          true,
		ShardCount:               4,
		ReservationLedgerMode:    "shadow",
		ReservationLedgerEnabled: true,
	}})
	if err != nil {
		t.Fatalf("authoritative inventory runtime rejected: %v", err)
	}
	if state.ShardCount != 4 {
		t.Fatalf("ShardCount=%d", state.ShardCount)
	}
}

func TestInventoryRuntimeReadinessRequiresReservationLedger(t *testing.T) {
	_, err := checkInventoryRuntime(context.Background(), runtimeStateClientStub{state: inventoryclient.RuntimeState{
		FinalDeductEnabled: true,
		RedisConfigured:    true,
		MySQLConfigured:    true,
		ShardCount:         4,
	}})
	if err == nil {
		t.Fatal("readiness must reject inventory runtime with reservation ledger disabled")
	}
}
