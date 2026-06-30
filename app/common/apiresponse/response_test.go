package apiresponse

import (
	"testing"

	"flash-mall/app/common/apperror"
)

func TestOK(t *testing.T) {
	resp := OK("req-1", map[string]string{"ok": "true"})
	if resp.Code != apperror.CodeOK || resp.RequestID != "req-1" || resp.Data == nil {
		t.Fatalf("unexpected OK response: %+v", resp)
	}
}

func TestFail(t *testing.T) {
	resp := Fail("req-2", apperror.New(apperror.CodeOrderStatusInvalid, "bad status"))
	if resp.Code != apperror.CodeOrderStatusInvalid || resp.Message != "bad status" || resp.RequestID != "req-2" {
		t.Fatalf("unexpected fail response: %+v", resp)
	}
}

func TestFailNilReturnsOK(t *testing.T) {
	resp := Fail("req-3", nil)
	if resp.Code != apperror.CodeOK {
		t.Fatalf("Code = %s, want %s", resp.Code, apperror.CodeOK)
	}
}
