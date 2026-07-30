package svc

import (
	"testing"

	"flash-mall/app/order/rpc/internal/config"
)

func TestBuildPaymentProviderDefaultsToLocalSandbox(t *testing.T) {
	provider, err := buildPaymentProvider(config.Config{})
	if err != nil {
		t.Fatalf("buildPaymentProvider() error = %v", err)
	}
	if provider != nil {
		t.Fatalf("local sandbox provider = %#v, want nil", provider)
	}
}

func TestBuildPaymentProviderRejectsUnknownMode(t *testing.T) {
	if _, err := buildPaymentProvider(config.Config{PaymentProvider: "unknown"}); err == nil {
		t.Fatal("buildPaymentProvider() accepted an unknown provider")
	}
}
