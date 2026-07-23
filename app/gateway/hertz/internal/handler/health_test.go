package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/assetstore"
	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type integrityReporterStub struct {
	snapshot assetstore.IntegritySnapshot
}

func (s integrityReporterStub) Snapshot() assetstore.IntegritySnapshot {
	return s.snapshot
}

type runtimeStateClientStub struct {
	state ports.InventoryRuntimeState
	err   error
}

func (s runtimeStateClientStub) GetRuntimeState(context.Context, ports.RequestMeta) (ports.InventoryRuntimeState, error) {
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

func TestLivenessDoesNotDependOnInventoryReadiness(t *testing.T) {
	h := server.Default()
	h.GET("/live", LivenessHandler(&svc.ServiceContext{Config: config.Config{
		Name:                   "gateway-test",
		InventoryKitexEndpoint: "inventory-kitex:8093",
	}}, time.Now()))

	resp := ut.PerformRequest(h.Engine, "GET", "/live", nil).Result()
	if resp.StatusCode() != consts.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestReadinessReportsMissingHistoricalAssetsAsDegraded(t *testing.T) {
	h := server.Default()
	h.GET("/ready", HealthHandler(&svc.ServiceContext{
		Config: config.Config{Name: "gateway-test"},
		UploadIntegrity: integrityReporterStub{snapshot: assetstore.IntegritySnapshot{
			CheckedAt: time.Now(),
			Report: assetstore.IntegrityReport{
				Configured:   true,
				Writable:     true,
				MissingFiles: 1,
			},
		}},
	}, time.Now()))

	resp := ut.PerformRequest(h.Engine, "GET", "/ready", nil).Result()
	if resp.StatusCode() != consts.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
	if !containsJSONText(resp.Body(), `"status":"degraded"`) {
		t.Fatalf("missing degraded status: %s", resp.Body())
	}
}

func TestReadinessRejectsConfiguredStoreWithoutIntegrityMonitor(t *testing.T) {
	h := server.Default()
	h.GET("/ready", HealthHandler(&svc.ServiceContext{
		Config:     config.Config{Name: "gateway-test"},
		AssetStore: assetstore.NewFilesystem(t.TempDir()),
	}, time.Now()))

	resp := ut.PerformRequest(h.Engine, "GET", "/ready", nil).Result()
	if resp.StatusCode() != consts.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func containsJSONText(body []byte, text string) bool {
	return strings.Contains(string(body), text)
}

func TestInventoryRuntimeReadinessRequiresAuthoritativeDeduct(t *testing.T) {
	_, err := checkInventoryRuntime(context.Background(), runtimeStateClientStub{state: ports.InventoryRuntimeState{
		RedisConfigured: true,
		MySQLConfigured: true,
		ShardCount:      4,
	}})
	if err == nil {
		t.Fatal("readiness must reject inventory runtime with final deduct disabled")
	}
}

func TestInventoryRuntimeReadinessAcceptsAuthoritativeRuntime(t *testing.T) {
	state, err := checkInventoryRuntime(context.Background(), runtimeStateClientStub{state: ports.InventoryRuntimeState{
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
	_, err := checkInventoryRuntime(context.Background(), runtimeStateClientStub{state: ports.InventoryRuntimeState{
		FinalDeductEnabled: true,
		RedisConfigured:    true,
		MySQLConfigured:    true,
		ShardCount:         4,
	}})
	if err == nil {
		t.Fatal("readiness must reject inventory runtime with reservation ledger disabled")
	}
}
