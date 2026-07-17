package handler

import (
	"context"
	"testing"
	"time"

	gatewaycache "flash-mall/app/gateway/hertz/internal/cache"
	"flash-mall/app/gateway/hertz/internal/svc"
)

func TestLoadCachedJSONPreservesTypedResponse(t *testing.T) {
	svcCtx := &svc.ServiceContext{Cache: gatewaycache.New(gatewaycache.Config{EnableL1: true, L1TTL: time.Minute}, nil)}
	loads := 0
	loader := func(context.Context) (PublicStoreDetail, error) {
		loads++
		return PublicStoreDetail{MerchantID: 7, MerchantName: "store"}, nil
	}
	first, _, err := loadCachedJSON(context.Background(), svcCtx, "store:7", loader)
	if err != nil {
		t.Fatal(err)
	}
	second, source, err := loadCachedJSON(context.Background(), svcCtx, "store:7", loader)
	if err != nil || source != gatewaycache.SourceL1 || first != second || loads != 1 {
		t.Fatalf("first=%+v second=%+v source=%s loads=%d err=%v", first, second, source, loads, err)
	}
}
