package inventoryclient

import (
	"context"
	"testing"

	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"
)

func TestRequestMetaCarriesTraceAndIdentity(t *testing.T) {
	ctx := tracectx.WithTrace(context.Background(), tracectx.Trace{RequestID: "req-1", TraceID: "trace-1"})
	ctx = authctx.WithIdentity(ctx, authctx.Identity{UserID: 7, MerchantID: 9, Role: authctx.RoleMerchant})

	meta := requestMeta(ctx)
	if meta.GetRequestId() != "req-1" || meta.GetTraceId() != "trace-1" {
		t.Fatalf("unexpected trace metadata: %+v", meta)
	}
	if meta.GetUserId() != 7 || meta.GetMerchantId() != 9 || meta.GetRole() != string(authctx.RoleMerchant) {
		t.Fatalf("unexpected identity metadata: %+v", meta)
	}
}

func TestRequestMetaGeneratesRequestID(t *testing.T) {
	meta := requestMeta(context.Background())
	if meta.GetRequestId() == "" || meta.GetTraceId() == "" {
		t.Fatalf("generated metadata must be traceable: %+v", meta)
	}
}
