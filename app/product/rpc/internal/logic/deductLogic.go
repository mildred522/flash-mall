package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"

	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeductLogic {
	return &DeductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Deduct 扣减库存（SAGA 正向操作）
func (l *DeductLogic) Deduct(in *product.DeductReq) (*product.Empty, error) {
	span := trace.SpanFromContext(l.ctx)
	span.SetAttributes(
		attribute.Int64("product.id", in.GetId()),
		attribute.Int64("stock.amount", in.GetNum()),
		attribute.String("order.id", in.GetOrderId()),
	)

	// 1) 获取 DTM Barrier
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "DTM Barrier 获取失败")
	}

	// 2) 获取原生 DB 连接
	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "DB 连接获取失败")
	}

	bucketCount := stockBucketCount(l.svcCtx.Config.StockBucketCount)
	orderId := in.OrderId
	if orderId == "" {
		// CHG 2026-02-24: 变更=空 order_id 使用商品 id 兜底; 之前=直接哈希空串; 原因=避免桶路由不稳定。
		orderId = strconv.FormatInt(in.Id, 10)
	}
	bucketIdx := stockBucketIndex(orderId, bucketCount)
	span.SetAttributes(attribute.Int("stock.bucket_idx", bucketIdx))

	// 3) 在 Barrier 保护下执行本地事务
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		l.Infow("deduct stock start",
			logx.Field("product_id", in.Id),
			logx.Field("amount", in.Num),
			logx.Field("bucket_idx", bucketIdx),
		)

		// CHG 2026-02-24: 变更=扣减落到分桶表; 之前=单行 product 表扣减; 原因=降低热点行冲突。
		actualBucketIdx, err := lockDeductibleStockBucket(tx, in.Id, in.Num, bucketIdx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(
			"UPDATE product_stock_bucket SET stock = stock - ?, version = version + 1 WHERE product_id = ? AND bucket_idx = ? AND stock >= ?",
			in.Num, in.Id, actualBucketIdx, in.Num,
		); err != nil {
			return err
		}
		_, err = tx.Exec("INSERT IGNORE INTO stock_log (order_id, type) VALUES (?, ?)", orderId, stockDeductBucketLogType(actualBucketIdx))
		return err
	})
	if err != nil {
		return nil, err
	}

	return &product.Empty{}, nil
}

func lockDeductibleStockBucket(tx *sql.Tx, productID int64, amount int64, preferredBucketIdx int) (int, error) {
	var bucketIdx int
	err := tx.QueryRow(
		`SELECT bucket_idx
		 FROM product_stock_bucket
		 WHERE product_id = ? AND stock >= ?
		 ORDER BY CASE WHEN bucket_idx = ? THEN 0 ELSE 1 END, bucket_idx
		 LIMIT 1
		 FOR UPDATE`,
		productID, amount, preferredBucketIdx,
	).Scan(&bucketIdx)
	if err == nil {
		return bucketIdx, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	var exists int
	err = tx.QueryRow("SELECT 1 FROM product_stock_bucket WHERE product_id = ? LIMIT 1", productID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, status.Error(codes.NotFound, "库存分桶不存在")
		}
		return 0, err
	}
	return 0, status.Error(codes.Aborted, "库存不足")
}

func stockDeductBucketLogType(bucketIdx int) string {
	return fmt.Sprintf("DEDUCT_BUCKET_%d", bucketIdx)
}
