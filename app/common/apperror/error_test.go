package apperror

import (
	"errors"
	"testing"
)

func TestFromErrorKeepsAppError(t *testing.T) {
	want := New(CodeStockInsufficient, "stock is not enough")
	got := FromError(want)
	if got != want {
		t.Fatalf("expected original app error")
	}
}

func TestFromErrorWrapsUnknownError(t *testing.T) {
	cause := errors.New("boom")
	got := FromError(cause)
	if got.Code != CodeInternal {
		t.Fatalf("Code = %s, want %s", got.Code, CodeInternal)
	}
	if !errors.Is(got, cause) {
		t.Fatalf("wrapped error does not expose cause")
	}
}
