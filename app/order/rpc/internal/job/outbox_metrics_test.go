package job

import "testing"

func TestNormalizeOutboxEventTypeIsBounded(t *testing.T) {
	for input, want := range map[string]string{
		"order.created":  "order.created",
		"order.paid":     "order.paid",
		"order.refunded": "order.refunded",
		"future.event":   "other",
		"":               "other",
	} {
		if got := normalizeOutboxEventType(input); got != want {
			t.Fatalf("normalizeOutboxEventType(%q)=%q, want %q", input, got, want)
		}
	}
}
