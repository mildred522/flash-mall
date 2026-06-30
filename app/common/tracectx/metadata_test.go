package tracectx

import "testing"

func TestTraceMetadataRoundTrip(t *testing.T) {
	trace := Trace{RequestID: "req-1", TraceID: "trace-1", UserID: "100", MerchantID: "200"}
	got := FromMetadata(trace.ToMetadata())
	if got != trace {
		t.Fatalf("Trace = %+v, want %+v", got, trace)
	}
}

func TestNewRequestID(t *testing.T) {
	id := NewRequestID()
	if len(id) <= len("req-") || id[:4] != "req-" {
		t.Fatalf("unexpected request id: %q", id)
	}
}
