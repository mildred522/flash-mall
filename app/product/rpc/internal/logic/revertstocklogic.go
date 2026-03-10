package logic

import (
	"context"
	"database/sql"
	"strconv"

	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevertStockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevertStockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevertStockLogic {
	return &RevertStockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RevertStock 归还库存（用于超时关单等场景）
func (l *RevertStockLogic) RevertStock(in *product.RevertStockReq) (*product.RevertStockResp, error) {
	// 1) 获取 RawDB（手动事务）
	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, err
	}

	// 2) 开启本地事务
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	bucketCount := stockBucketCount(l.svcCtx.Config.StockBucketCount)
	orderId := in.OrderId
	if orderId == "" {
		// CHG 2026-02-24: 变更=空 order_id 使用商品 id 兜底; 之前=直接哈希空串; 原因=保证分桶回滚稳定。
		orderId = strconv.FormatInt(in.Id, 10)
	}
	bucketIdx := stockBucketIndex(orderId, bucketCount)

	// 3) 手动屏障：插入流水记录（order_id + type 唯一）
	_, err = tx.ExecContext(l.ctx, "INSERT INTO stock_log (order_id, type) VALUES (?, 'REVERT')", in.OrderId)
	if err != nil {
		// 重复说明已处理过，直接返回成功
		return &product.RevertStockResp{}, nil
	}

	// 4) 执行业务：归还库存
	// CHG 2026-02-24: 变更=归还落到分桶表; 之前=单行 product 表回补; 原因=与扣减保持一致的分桶路由。
	_, err = tx.ExecContext(
		l.ctx,
		"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) "+
			"ON DUPLICATE KEY UPDATE stock = stock + VALUES(stock), version = version + 1",
		in.Id, bucketIdx, in.Num,
	)
	if err != nil {
		return nil, err
	}

	// 5) 提交事务
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &product.RevertStockResp{}, nil
}
