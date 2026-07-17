package handler

import (
	"context"
	"database/sql"
	"sync"
)

type storefrontSchemaState struct {
	mu    sync.Mutex
	ready bool
}

var (
	storefrontSchemaStates           sync.Map
	merchantStoreProfileSchemaStates sync.Map
)

func requireStorefrontSchema(ctx context.Context, db *sql.DB) error {
	value, _ := storefrontSchemaStates.LoadOrStore(db, &storefrontSchemaState{})
	state := value.(*storefrontSchemaState)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		return nil
	}
	if err := requireMerchantStoreProfileSchema(ctx, db); err != nil {
		return err
	}
	if err := requireSchemaColumn(ctx, db, "mall_product", "product", "create_time"); err != nil {
		return err
	}
	if err := requireSchemaTable(ctx, db, "mall_product", "homepage_showcase"); err != nil {
		return err
	}
	if err := requireSchemaTable(ctx, db, "mall_product", "homepage_showcase_item"); err != nil {
		return err
	}
	state.ready = true
	return nil
}

func requireMerchantStoreProfileSchema(ctx context.Context, db *sql.DB) error {
	value, _ := merchantStoreProfileSchemaStates.LoadOrStore(db, &storefrontSchemaState{})
	state := value.(*storefrontSchemaState)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		return nil
	}
	if err := requireSchemaTable(ctx, db, "mall_order", "merchant_store_profile"); err != nil {
		return err
	}
	state.ready = true
	return nil
}
