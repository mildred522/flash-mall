package observability

import "testing"

func TestTracingConfigEnabled(t *testing.T) {
	cfg := TracingConfig{}
	if cfg.IsEnabled() {
		t.Fatal("zero tracing config should be disabled")
	}

	cfg.Enabled = true
	cfg.ServiceName = "order-api"
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
