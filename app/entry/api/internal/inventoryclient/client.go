package inventoryclient

import (
	"context"

	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"

	kitexclient "github.com/cloudwego/kitex/client"
)

type Client interface {
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error
	ReconcileStock(ctx context.Context, productID int64) (*inventory.ReconcileStockResponse, error)
}

type KitexClient struct {
	client inventoryservice.Client
}

func NewKitexClient(endpoint string) (*KitexClient, error) {
	client, err := inventoryservice.NewClient("InventoryService", kitexclient.WithHostPorts(endpoint))
	if err != nil {
		return nil, err
	}
	return &KitexClient{client: client}, nil
}

func (c *KitexClient) SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error {
	var shardCountPtr *int32
	if shardCount > 0 {
		value := int32(shardCount)
		shardCountPtr = &value
	}
	_, err := c.client.SeedStock(ctx, &inventory.SeedStockRequest{ProductId: productID, Total: total, ShardCount: shardCountPtr})
	return err
}

func (c *KitexClient) ReconcileStock(ctx context.Context, productID int64) (*inventory.ReconcileStockResponse, error) {
	return c.client.ReconcileStock(ctx, &inventory.ReconcileStockRequest{ProductId: productID})
}
