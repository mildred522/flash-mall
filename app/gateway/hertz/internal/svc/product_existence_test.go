package svc

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/config"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
)

type emptyProductIDSource struct{}

func (emptyProductIDSource) ProductIDBatch(context.Context, int64, int) ([]int64, error) {
	return []int64{}, nil
}

func TestConfigureProductExistenceRequiresExplicitEnablement(t *testing.T) {
	svcCtx := &ServiceContext{}
	if got := configureProductExistence(svcCtx, nil, emptyProductIDSource{}); got != nil {
		t.Fatal("disabled filter should not be configured")
	}
	if svcCtx.ProductExistenceFilter != nil || svcCtx.ProductNegativeCache != nil {
		t.Fatal("disabled filter should leave service fields empty")
	}
}

func TestConfigureProductExistenceSharesConfiguredRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	svcCtx := &ServiceContext{Config: config.Config{ProductExistenceFilterEnabled: true}}
	if got := configureProductExistence(svcCtx, client, emptyProductIDSource{}); got == nil {
		t.Fatal("enabled filter should be configured")
	}
	if svcCtx.ProductExistenceFilter == nil || svcCtx.ProductNegativeCache == nil {
		t.Fatal("enabled filter should expose filter and negative cache")
	}
}
