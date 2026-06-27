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
