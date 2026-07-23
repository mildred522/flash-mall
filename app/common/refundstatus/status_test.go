package refundstatus

import "testing"

func TestText(t *testing.T) {
	tests := map[int64]string{
		Requested: "requested",
		Approved:  "approved",
		Success:   "success",
		Rejected:  "rejected",
		Failed:    "failed",
		99:        "unknown",
	}
	for statusCode, want := range tests {
		if got := Text(statusCode); got != want {
			t.Fatalf("Text(%d)=%q, want %q", statusCode, got, want)
		}
	}
}
