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
