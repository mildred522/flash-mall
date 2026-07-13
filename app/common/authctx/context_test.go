package authctx

import (
	"context"
	"testing"
)

func TestIdentityContext(t *testing.T) {
	identity := Identity{UserID: 100, Role: RoleMerchant, MerchantID: 200, RequestID: "req-1"}
	ctx := WithIdentity(context.Background(), identity)
	got, ok := IdentityFrom(ctx)
	if !ok || got.UserID != identity.UserID || got.MerchantID != identity.MerchantID {
		t.Fatalf("identity not round-tripped: %+v ok=%v", got, ok)
	}
	if !got.HasMerchant() {
		t.Fatalf("expected merchant identity")
	}
}

func TestCanAdmin(t *testing.T) {
	if !(Identity{Role: RoleAdmin}).CanAdmin() {
		t.Fatalf("admin role should be allowed")
	}
	if !(Identity{IsAdmin: true}).CanAdmin() {
		t.Fatalf("admin flag should be allowed")
	}
}
