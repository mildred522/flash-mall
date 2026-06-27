# Trading Loop V4 Phase A-B Observability Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the V4 baseline by merging Trading Loop V3, fixing CI governance for the active frontend workspace, and adding configurable OpenTelemetry foundation for the synchronous order flow.

**Architecture:** Phase A makes `main` a correct implementation baseline by merging V3 and fixing GitHub Actions. Phase B adds a small shared observability package used by entry-api, order-rpc, and product-rpc; tracing stays config-gated and additive. RabbitMQ async trace headers, performance workflow, and playbooks are intentionally left for a later Phase C-D plan.

**Tech Stack:** Go 1.24, go-zero REST/RPC, gRPC, OpenTelemetry Go SDK, Prometheus client, MySQL, Redis, RabbitMQ, React/Vite frontend, GitHub Actions.

---

## Scope Boundaries

This plan implements V4 Phase A-B only:

1. Merge `codex/trading-loop-v3` into `main`.
2. Fix CI to build the active `frontend` workspace.
3. Add shared observability config/runtime helpers.
4. Enable configurable OpenTelemetry in entry-api, order-rpc, and product-rpc.
5. Add basic correlation helpers and critical synchronous flow instrumentation.
6. Add local Jaeger support for demonstration.

This plan does not implement:

1. RabbitMQ trace headers and consumer extraction.
2. Outbox publish/consume metrics with event-type labels.
3. Manual performance regression GitHub Actions workflow.
4. Observability playbooks.

Those belong in a follow-up Phase C-D plan after this baseline is green.

## File Map

Create:

- `app/common/observability/config.go`: shared observability config structs.
- `app/common/observability/tracing.go`: OpenTelemetry provider setup and shutdown.
- `app/common/observability/diagnostics.go`: shared metrics/pprof HTTP server startup.
- `app/common/observability/correlation.go`: `TraceFields` and `OrderFields` helpers.
- `app/common/observability/tracing_test.go`: config defaults and disabled tracing tests.
- `app/common/observability/correlation_test.go`: trace field extraction tests.

Modify:

- `.github/workflows/ci.yml`: active frontend cache/build paths and focused verification commands.
- `deploy/docker-compose.yml`: optional local Jaeger service for trace demo.
- `app/entry/api/internal/config/config.go`: add shared observability config.
- `app/entry/api/etc/entry-api.yaml`: add disabled-by-default V4 tracing config.
- `app/entry/api/entry.go`: initialize tracing and shared diagnostics.
- `app/entry/api/internal/logic/createorderlogic.go`: add correlation fields around create-order logs.
- `app/order/rpc/internal/config/config.go`: add shared observability config.
- `app/order/rpc/etc/order.yaml`: add disabled-by-default V4 tracing config.
- `app/order/rpc/order.go`: initialize tracing and shared diagnostics.
- `app/order/rpc/internal/logic/createOrderLogic.go`: add create-order span attributes and correlation fields.
- `app/product/rpc/internal/config/config.go`: add `PprofAddr`, `MetricsAddr`, and shared observability config.
- `app/product/rpc/etc/product.yaml`: add metrics/pprof ports and disabled-by-default V4 tracing config.
- `app/product/rpc/product.go`: initialize tracing and diagnostics.
- `app/product/rpc/internal/logic/deductLogic.go`: add stock-operation span attributes and correlation fields.
- `k8s/apps/01-configmaps.yaml`: add V4 tracing config and product-rpc metrics/pprof config.
- `k8s/apps/04-product-rpc.yaml`: expose product-rpc metrics/pprof ports if not already present.

Generated files should not be edited in this phase.

---

### Task 0: Merge Trading Loop V3 Baseline

**Files:**
- Merge source: branch `codex/trading-loop-v3`
- Target: `main`

- [ ] **Step 1: Confirm clean working tree**

Run:

```powershell
git status --short
```

Expected: only known untracked local scratch paths may appear, such as `.codex_tmp/`. There must be no staged or modified tracked files.

- [ ] **Step 2: Merge V3 into main**

Run:

```powershell
git switch main
git merge --no-ff codex/trading-loop-v3 -m "merge: trading loop v3 lifecycle baseline"
```

Expected: merge succeeds or reports explicit conflicts. If conflicts occur, resolve them preserving V3 lifecycle RPC/API/frontend behavior from `codex/trading-loop-v3`.

- [ ] **Step 3: Verify merged backend**

Run:

```powershell
go test ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... -count=1
goctl api validate -api app/entry/api/entry.api
```

Expected: all Go packages pass and `goctl` prints `api format ok`.

- [ ] **Step 4: Verify merged frontend**

Run:

```powershell
npm run build:shop --prefix frontend
npm run build:admin --prefix frontend
```

Expected: both Vite builds exit 0.

- [ ] **Step 5: Commit only if the merge required conflict resolution**

If `git merge` already created the merge commit, do not create another commit. If conflict resolution left files staged but no commit was created, run:

```powershell
git add app frontend scripts k8s docs
git commit -m "merge: trading loop v3 lifecycle baseline"
```

Expected: `main` now contains V3 commits and passes the verification commands above.

---

### Task 1: Fix CI Frontend Baseline

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Update Node cache path and frontend build steps**

Replace both frontend build blocks in `.github/workflows/ci.yml` with this shape:

```yaml
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Build frontend
        run: cd frontend && npm ci && npm run build:shop && npm run build:admin
```

Expected: no remaining references to `web/package-lock.json` or `cd web`.

- [ ] **Step 2: Narrow Go verification to active services**

In the `go-checks` job, replace:

```yaml
      - name: Test
        run: go test ./...
```

with:

```yaml
      - name: Test active services
        run: go test ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... ./app/auth/api/... -count=1

      - name: Validate entry API contract
        run: |
          go install github.com/zeromicro/go-zero/tools/goctl@v1.9.2
          goctl api validate -api app/entry/api/entry.api
```

Expected: CI validates the services used by this project and validates the entry API contract.

- [ ] **Step 3: Run local equivalent checks**

Run:

```powershell
npm ci --prefix frontend
npm run build:shop --prefix frontend
npm run build:admin --prefix frontend
go test ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... ./app/auth/api/... -count=1
goctl api validate -api app/entry/api/entry.api
```

Expected: all commands exit 0.

- [ ] **Step 4: Commit CI fix**

Run:

```powershell
git add .github/workflows/ci.yml
git commit -m "fix: align ci with active frontend workspace"
```

Expected: a focused CI commit.

---

### Task 2: Add Shared Observability Config and Diagnostics Helpers

**Files:**
- Create: `app/common/observability/config.go`
- Create: `app/common/observability/diagnostics.go`
- Create: `app/common/observability/tracing_test.go`

- [ ] **Step 1: Write disabled-config tests**

Create `app/common/observability/tracing_test.go`:

```go
package observability

import "testing"

func TestTracingConfigEnabled(t *testing.T) {
	cfg := TracingConfig{}
	if cfg.IsEnabled() {
		t.Fatal("zero tracing config should be disabled")
	}

	cfg.Enabled = true
	cfg.ServiceName = "entry-api"
	if !cfg.IsEnabled() {
		t.Fatal("enabled tracing config with service name should be enabled")
	}
}

func TestTracingConfigSampleRatio(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"negative", -1, 0},
		{"zero", 0, 0},
		{"middle", 0.25, 0.25},
		{"too high", 2, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := TracingConfig{SampleRatio: tt.in}
			if got := cfg.NormalizedSampleRatio(); got != tt.want {
				t.Fatalf("NormalizedSampleRatio()=%v want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run red test**

Run:

```powershell
go test ./app/common/observability -run "TestTracingConfig" -count=1
```

Expected: FAIL because `TracingConfig` is undefined.

- [ ] **Step 3: Add config types**

Create `app/common/observability/config.go`:

```go
package observability

import "strings"

type Config struct {
	Tracing TracingConfig
}

type TracingConfig struct {
	Enabled     bool
	ServiceName string
	Exporter    string
	Endpoint    string
	SampleRatio float64
}

func (c TracingConfig) IsEnabled() bool {
	return c.Enabled && strings.TrimSpace(c.ServiceName) != ""
}

func (c TracingConfig) NormalizedSampleRatio() float64 {
	if c.SampleRatio <= 0 {
		return 0
	}
	if c.SampleRatio >= 1 {
		return 1
	}
	return c.SampleRatio
}

func (c TracingConfig) normalizedExporter() string {
	exporter := strings.ToLower(strings.TrimSpace(c.Exporter))
	if exporter == "" {
		return "stdout"
	}
	return exporter
}
```

- [ ] **Step 4: Add diagnostics helper**

Create `app/common/observability/diagnostics.go`:

```go
package observability

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeromicro/go-zero/core/logx"
)

func StartDiagnostics(metricsAddr, pprofAddr string) {
	if metricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		go func() {
			if err := http.ListenAndServe(metricsAddr, mux); err != nil {
				logx.Errorf("metrics server stopped: addr=%s err=%v", metricsAddr, err)
			}
		}()
	}
	if pprofAddr != "" {
		go func() {
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				logx.Errorf("pprof server stopped: addr=%s err=%v", pprofAddr, err)
			}
		}()
	}
}
```

- [ ] **Step 5: Run green test**

Run:

```powershell
go test ./app/common/observability -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit config and diagnostics helpers**

Run:

```powershell
git add app/common/observability/config.go app/common/observability/diagnostics.go app/common/observability/tracing_test.go
git commit -m "feat: add shared observability config helpers"
```

Expected: focused helper commit.

---

### Task 3: Add OpenTelemetry Provider Setup

**Files:**
- Create: `app/common/observability/tracing.go`
- Modify: `app/common/observability/tracing_test.go`

- [ ] **Step 1: Add setup tests**

Append to `app/common/observability/tracing_test.go`:

```go
func TestSetupTracingDisabled(t *testing.T) {
	shutdown, err := SetupTracing(t.Context(), TracingConfig{})
	if err != nil {
		t.Fatalf("SetupTracing disabled returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected no-op shutdown")
	}
	if err := shutdown(t.Context()); err != nil {
		t.Fatalf("disabled shutdown returned error: %v", err)
	}
}

func TestSetupTracingStdout(t *testing.T) {
	shutdown, err := SetupTracing(t.Context(), TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter:    "stdout",
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("SetupTracing stdout returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown")
	}
	if err := shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run red test**

Run:

```powershell
go test ./app/common/observability -run "TestSetupTracing" -count=1
```

Expected: FAIL because `SetupTracing` is undefined.

- [ ] **Step 3: Implement tracing provider setup**

Create `app/common/observability/tracing.go`:

```go
package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type ShutdownFunc func(context.Context) error

func SetupTracing(ctx context.Context, cfg TracingConfig) (ShutdownFunc, error) {
	if !cfg.IsEnabled() {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.NormalizedSampleRatio())),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider.Shutdown, nil
}

func newTraceExporter(ctx context.Context, cfg TracingConfig) (sdktrace.SpanExporter, error) {
	switch cfg.normalizedExporter() {
	case "stdout":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "otlphttp", "otlp-http":
		opts := []otlptracehttp.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpointURL(cfg.Endpoint))
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("unsupported trace exporter: %s", cfg.Exporter)
	}
}
```

- [ ] **Step 4: Run green test**

Run:

```powershell
go test ./app/common/observability -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit tracing setup**

Run:

```powershell
git add app/common/observability/tracing.go app/common/observability/tracing_test.go
git commit -m "feat: add configurable tracing provider"
```

Expected: tracing setup committed.

---

### Task 4: Add Correlation Helpers

**Files:**
- Create: `app/common/observability/correlation.go`
- Create: `app/common/observability/correlation_test.go`

- [ ] **Step 1: Write correlation tests**

Create `app/common/observability/correlation_test.go`:

```go
package observability

import (
	"context"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel"
)

func TestTraceFieldsWithoutSpan(t *testing.T) {
	fields := TraceFields(context.Background())
	if len(fields) != 0 {
		t.Fatalf("expected no fields, got %#v", fields)
	}
}

func TestOrderFieldsIncludesBusinessFields(t *testing.T) {
	ctx, span := otel.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	fields := OrderFields(ctx, "order-1", "req-1")
	names := map[string]bool{}
	for _, field := range fields {
		names[field.Key] = true
	}

	for _, key := range []string{"trace_id", "span_id", "order_id", "request_id"} {
		if !names[key] {
			t.Fatalf("missing field %s in %#v", key, fields)
		}
	}
}

func TestBusinessFieldsSkipEmptyValues(t *testing.T) {
	fields := append([]logx.LogField{}, OrderFields(context.Background(), "", "")...)
	if len(fields) != 0 {
		t.Fatalf("expected no fields, got %#v", fields)
	}
}
```

- [ ] **Step 2: Run red test**

Run:

```powershell
go test ./app/common/observability -run "TestTraceFields|TestOrderFields|TestBusinessFields" -count=1
```

Expected: FAIL because helper functions are undefined.

- [ ] **Step 3: Implement helpers**

Create `app/common/observability/correlation.go`:

```go
package observability

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/trace"
)

func TraceFields(ctx context.Context) []logx.LogField {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil
	}
	return []logx.LogField{
		logx.Field("trace_id", spanCtx.TraceID().String()),
		logx.Field("span_id", spanCtx.SpanID().String()),
	}
}

func OrderFields(ctx context.Context, orderID, requestID string) []logx.LogField {
	fields := TraceFields(ctx)
	if orderID != "" {
		fields = append(fields, logx.Field("order_id", orderID))
	}
	if requestID != "" {
		fields = append(fields, logx.Field("request_id", requestID))
	}
	return fields
}
```

- [ ] **Step 4: Run green test**

Run:

```powershell
go test ./app/common/observability -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit correlation helpers**

Run:

```powershell
git add app/common/observability/correlation.go app/common/observability/correlation_test.go
git commit -m "feat: add trace correlation log helpers"
```

Expected: correlation helpers committed.

---

### Task 5: Wire Observability Config into Services

**Files:**
- Modify: `app/entry/api/internal/config/config.go`
- Modify: `app/order/rpc/internal/config/config.go`
- Modify: `app/product/rpc/internal/config/config.go`
- Modify: `app/entry/api/etc/entry-api.yaml`
- Modify: `app/order/rpc/etc/order.yaml`
- Modify: `app/product/rpc/etc/product.yaml`
- Modify: `k8s/apps/01-configmaps.yaml`

- [ ] **Step 1: Add shared config fields**

Add imports and fields.

For `app/entry/api/internal/config/config.go`:

```go
import (
	commonobs "flash-mall/app/common/observability"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)
```

Add to `Config`:

```go
Observability commonobs.Config `json:",optional"`
```

Repeat the same `commonobs` import and `Observability commonobs.Config` field in `app/order/rpc/internal/config/config.go` and `app/product/rpc/internal/config/config.go`.

For product-rpc also add:

```go
PprofAddr   string
MetricsAddr string
```

- [ ] **Step 2: Add disabled local YAML config**

Append to `app/entry/api/etc/entry-api.yaml`, `app/order/rpc/etc/order.yaml`, and `app/product/rpc/etc/product.yaml`:

```yaml
Observability:
  Tracing:
    Enabled: false
    ServiceName: entry-api
    Exporter: stdout
    Endpoint:
    SampleRatio: 0
```

Use `ServiceName: order-rpc` in `order.yaml` and `ServiceName: product-rpc` in `product.yaml`.

Add to `app/product/rpc/etc/product.yaml`:

```yaml
PprofAddr: 0.0.0.0:6062
MetricsAddr: 0.0.0.0:9092
```

- [ ] **Step 3: Add K8s config**

In `k8s/apps/01-configmaps.yaml`, add matching disabled `Observability` blocks for entry-api, order-rpc, and product-rpc. Add `PprofAddr: 0.0.0.0:6062` and `MetricsAddr: 0.0.0.0:9092` to the product-rpc config map.

- [ ] **Step 4: Compile config users**

Run:

```powershell
go test ./app/entry/api/internal/config ./app/order/rpc/internal/config ./app/product/rpc/internal/config -count=1
```

Expected: packages compile, even if there are no test files.

- [ ] **Step 5: Commit config wiring**

Run:

```powershell
git add app/entry/api/internal/config/config.go app/order/rpc/internal/config/config.go app/product/rpc/internal/config/config.go app/entry/api/etc/entry-api.yaml app/order/rpc/etc/order.yaml app/product/rpc/etc/product.yaml k8s/apps/01-configmaps.yaml
git commit -m "feat: add observability config to services"
```

Expected: config commit with tracing disabled by default.

---

### Task 6: Initialize Tracing and Diagnostics in Service Entrypoints

**Files:**
- Modify: `app/entry/api/entry.go`
- Modify: `app/order/rpc/order.go`
- Modify: `app/product/rpc/product.go`
- Modify: `k8s/apps/04-product-rpc.yaml`

- [ ] **Step 1: Replace duplicated diagnostics startup**

In `app/entry/api/entry.go`, replace direct `/metrics` and pprof startup with:

```go
shutdownTracing, err := observability.SetupTracing(context.Background(), c.Observability.Tracing)
if err != nil {
	panic(err)
}
defer shutdownTracing(context.Background())

observability.StartDiagnostics(c.MetricsAddr, c.PprofAddr)
```

Required import changes:

```go
import (
	"context"
	"flag"
	"fmt"

	"flash-mall/app/common/observability"
	"flash-mall/app/entry/api/internal/config"
	"flash-mall/app/entry/api/internal/handler"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/job"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)
```

Remove `net/http`, `_ "net/http/pprof"`, and `prometheus/promhttp` imports from this file.

- [ ] **Step 2: Apply the same pattern to order-rpc**

In `app/order/rpc/order.go`, initialize tracing after config load and before creating the service context:

```go
shutdownTracing, err := observability.SetupTracing(context.Background(), c.Observability.Tracing)
if err != nil {
	panic(err)
}
defer shutdownTracing(context.Background())

ctx := svc.NewServiceContext(c)
job.NewOutboxPublisher(ctx).Start()
observability.StartDiagnostics(c.MetricsAddr, c.PprofAddr)
```

Required import additions:

```go
import (
	"context"
	"flag"
	"fmt"

	"flash-mall/app/common/observability"
)
```

Remove direct `net/http`, `_ "net/http/pprof"`, and `prometheus/promhttp` imports from this file.

- [ ] **Step 3: Add product-rpc tracing and diagnostics**

In `app/product/rpc/product.go`, initialize tracing after config load:

```go
shutdownTracing, err := observability.SetupTracing(context.Background(), c.Observability.Tracing)
if err != nil {
	panic(err)
}
defer shutdownTracing(context.Background())

ctx := svc.NewServiceContext(c)
observability.StartDiagnostics(c.MetricsAddr, c.PprofAddr)
```

Required import additions:

```go
import (
	"context"
	"flag"
	"fmt"

	"flash-mall/app/common/observability"
)
```

- [ ] **Step 4: Expose product-rpc metrics and pprof in Kubernetes**

In `k8s/apps/04-product-rpc.yaml`, ensure container ports include:

```yaml
            - name: metrics
              containerPort: 9092
            - name: pprof
              containerPort: 6062
```

If the manifest already contains these ports, leave it unchanged.

- [ ] **Step 5: Run entrypoint compile checks**

Run:

```powershell
go test ./app/entry/api ./app/order/rpc ./app/product/rpc -count=1
```

Expected: all three main packages compile.

- [ ] **Step 6: Commit service initialization**

Run:

```powershell
git add app/entry/api/entry.go app/order/rpc/order.go app/product/rpc/product.go k8s/apps/04-product-rpc.yaml
git commit -m "feat: initialize observability in service entrypoints"
```

Expected: service startup commit.

---

### Task 7: Add Critical Synchronous Flow Correlation

**Files:**
- Modify: `app/entry/api/internal/logic/createorderlogic.go`
- Modify: `app/order/rpc/internal/logic/createOrderLogic.go`
- Modify: `app/product/rpc/internal/logic/deductLogic.go`

- [ ] **Step 1: Add entry-api create-order correlation fields**

In `app/entry/api/internal/logic/createorderlogic.go`, import:

```go
commonobs "flash-mall/app/common/observability"
```

Where create-order logs currently include `request_id` and `order_id`, replace ad-hoc fields with:

```go
l.Infow("create order request accepted",
	commonobs.OrderFields(l.ctx, orderID, requestId)...,
)
```

For log calls before `orderID` exists, use:

```go
l.Infow("create order request received",
	commonobs.OrderFields(l.ctx, "", requestId)...,
)
```

- [ ] **Step 2: Add order-rpc create-order span attributes**

In `app/order/rpc/internal/logic/createOrderLogic.go`, import:

```go
commonobs "flash-mall/app/common/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
```

After request validation, add:

```go
span := trace.SpanFromContext(l.ctx)
span.SetAttributes(
	attribute.String("order.id", in.GetOrderId()),
	attribute.String("order.request_id", in.GetRequestId()),
	attribute.Int64("user.id", in.GetUserId()),
	attribute.Int64("product.id", in.GetProductId()),
	attribute.Int64("order.amount", in.GetAmount()),
)
```

Update key logs in this file to use:

```go
l.Infow("order rpc create order",
	commonobs.OrderFields(l.ctx, in.GetOrderId(), in.GetRequestId())...,
)
```

- [ ] **Step 3: Add product-rpc deduct span attributes**

In `app/product/rpc/internal/logic/deductLogic.go`, import:

```go
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
```

At the start of `Deduct`, add:

```go
span := trace.SpanFromContext(l.ctx)
span.SetAttributes(
	attribute.Int64("product.id", in.GetId()),
	attribute.Int64("stock.amount", in.GetNum()),
	attribute.String("order.id", in.GetOrderId()),
)
```

If this file computes a bucket index, also set:

```go
span.SetAttributes(attribute.Int("stock.bucket_idx", bucketIdx))
```

- [ ] **Step 4: Run focused compile checks**

Run:

```powershell
go test ./app/entry/api/internal/logic ./app/order/rpc/internal/logic ./app/product/rpc/internal/logic -count=1
```

Expected: all packages pass.

- [ ] **Step 5: Commit synchronous correlation**

Run:

```powershell
git add app/entry/api/internal/logic/createorderlogic.go app/order/rpc/internal/logic/createOrderLogic.go app/product/rpc/internal/logic/deductLogic.go
git commit -m "feat: add synchronous trading trace correlation"
```

Expected: focused instrumentation commit.

---

### Task 8: Add Local Jaeger Demo Support

**Files:**
- Modify: `deploy/docker-compose.yml`
- Modify: `app/entry/api/etc/entry-api.yaml`
- Modify: `app/order/rpc/etc/order.yaml`
- Modify: `app/product/rpc/etc/product.yaml`

- [ ] **Step 1: Add Jaeger service**

Add to `deploy/docker-compose.yml`:

```yaml
  jaeger:
    image: jaegertracing/all-in-one:1.57
    pull_policy: if_not_present
    container_name: jaeger
    ports:
      - "16686:16686"
      - "4318:4318"
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

- [ ] **Step 2: Keep tracing disabled by default but document local OTLP endpoint**

In each local YAML `Observability.Tracing` block, set:

```yaml
    Enabled: false
    Exporter: otlphttp
    Endpoint: http://127.0.0.1:4318/v1/traces
    SampleRatio: 1
```

Expected: developers can enable tracing by changing only `Enabled: true`.

- [ ] **Step 3: Validate local compose syntax**

Run:

```powershell
docker compose -f deploy/docker-compose.yml config
```

Expected: compose config renders successfully.

- [ ] **Step 4: Commit demo collector support**

Run:

```powershell
git add deploy/docker-compose.yml app/entry/api/etc/entry-api.yaml app/order/rpc/etc/order.yaml app/product/rpc/etc/product.yaml
git commit -m "chore: add local jaeger tracing demo config"
```

Expected: local demo config commit.

---

### Task 9: Full Phase A-B Verification and MCP Review

**Files:** all changed files.

- [ ] **Step 1: Run full backend checks**

Run:

```powershell
go test ./app/common/observability ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... ./app/auth/api/... -count=1
goctl api validate -api app/entry/api/entry.api
```

Expected: all tests pass and API contract validates.

- [ ] **Step 2: Run frontend checks**

Run:

```powershell
npm ci --prefix frontend
npm run build:shop --prefix frontend
npm run build:admin --prefix frontend
```

Expected: both frontend builds pass.

- [ ] **Step 3: Run smoke check**

Run:

```powershell
bash scripts/ci/smoke-e2e.sh
```

Expected: smoke test prints `smoke test passed with order_id=...`.

- [ ] **Step 4: Run local tracing demo check**

Run:

```powershell
docker compose -f deploy/docker-compose.yml up -d jaeger
```

Temporarily set `Observability.Tracing.Enabled: true` in local YAML files, start services with `scripts/local/start-all.ps1`, create one order, then open:

```text
http://127.0.0.1:16686
```

Expected: Jaeger shows spans for at least entry-api, order-rpc, and product-rpc. Revert the local YAML `Enabled` changes before committing unless the plan intentionally changes defaults.

- [ ] **Step 5: Review final diff with MCP**

Call `local_mcp_brain.review_diff` with the final non-generated diff. Ask it to focus on:

1. Tracing disabled-by-default behavior.
2. Prometheus label cardinality.
3. CI path correctness.
4. Product-rpc metrics/pprof compatibility.
5. Accidental business behavior changes.

Address valid findings before final commit.

- [ ] **Step 6: Commit verification notes if docs changed**

If verification adds a short execution note, commit it:

```powershell
git add docs
git commit -m "docs: record v4 phase ab verification"
```

Expected: no uncommitted tracked files remain after this step.

---

## Self-Review

Spec coverage:

- V3 prerequisite: Task 0.
- CI frontend path and baseline governance: Task 1.
- Shared observability foundation: Tasks 2-4.
- Service config and startup: Tasks 5-6.
- Synchronous trace/log correlation: Task 7.
- Local collector demo: Task 8.
- Verification and MCP review: Task 9.

Explicitly deferred:

- RabbitMQ trace headers and consumer extraction: Phase C plan.
- Outbox and lifecycle metrics expansion: Phase C plan.
- Manual performance workflow and playbooks: Phase D plan.

Completeness scan: every code-changing task names files, snippets, commands, and expected results.

Type consistency:

- Shared config type is `observability.Config`.
- Service config field is `Observability commonobs.Config`.
- Tracing setup function is `observability.SetupTracing`.
- Diagnostics setup function is `observability.StartDiagnostics`.
- Log helper functions are `observability.TraceFields` and `observability.OrderFields`.
