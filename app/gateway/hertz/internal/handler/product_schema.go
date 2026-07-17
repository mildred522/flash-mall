package handler

import (
	"context"
	"database/sql"
)

func requireGatewayProductReadSchema(ctx context.Context, db *sql.DB) error {
	if err := requireSchemaTable(ctx, db, "mall_product", "product_stock_snapshot"); err != nil {
		return err
	}
	return requireSchemaTable(ctx, db, "mall_product", "product_card_snapshot")
}

func requireGatewayProductMerchantSchema(ctx context.Context, db *sql.DB) error {
	if err := requireGatewayMerchantBaseSchema(ctx, db); err != nil {
		return err
	}
	if err := requireGatewayProductMerchantColumn(ctx, db); err != nil {
		return err
	}
	if err := requireGatewayProductImageColumn(ctx, db); err != nil {
		return err
	}
	if err := requireSchemaTable(ctx, db, "mall_product", "product_inventory_seed"); err != nil {
		return err
	}
	return requireGatewayProductReadSchema(ctx, db)
}

func requireGatewayMerchantBaseSchema(ctx context.Context, db *sql.DB) error {
	return requireSchemaTable(ctx, db, "mall_order", "merchant")
}

func requireGatewayProductMerchantColumn(ctx context.Context, db *sql.DB) error {
	return requireSchemaColumn(ctx, db, "mall_product", "product", "merchant_id")
}

func requireGatewayProductImageColumn(ctx context.Context, db *sql.DB) error {
	return requireSchemaColumn(ctx, db, "mall_product", "product", "image_url")
}
