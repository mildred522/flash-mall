package logic

import "testing"

func TestNormalizePaymentTransitionResultIsBounded(t *testing.T) {
	if got := normalizePaymentTransitionResult("IDEMPOTENT"); got != "idempotent" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePaymentTransitionResult("provider-specific"); got != "error" {
		t.Fatalf("unknown result=%q", got)
	}
}
