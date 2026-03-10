package logic

import (
	"context"
	"database/sql"
	"strconv"

	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"

	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeductRollbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeductRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeductRollbackLogic {
	return &DeductRollbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeductRollback 回滚库存（SAGA 补偿操作）
func (l *DeductRollbackLogic) DeductRollback(in *product.DeductReq) (*product.Empty, error) {
	// 1) 获取 DTM Barrier
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "DTM Barrier 获取失败")
	}

	// 2) 获取 DB
	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "DB 连接获取失败")
	}

	bucketCount := stockBucketCount(l.svcCtx.Config.StockBucketCount)
	orderId := in.OrderId
	if orderId == "" {
		// CHG 2026-02-24: 变更=空 order_id 使用商品 id 兜底; 之前=直接哈希空串; 原因=回滚与正向保持一致桶路由。
		orderId = strconv.FormatInt(in.Id, 10)
	}
	bucketIdx := stockBucketIndex(orderId, bucketCount)

	// 3) 执行回滚事务
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		l.Infow("deduct rollback start",
			logx.Field("product_id", in.Id),
			logx.Field("amount", in.Num),
			logx.Field("bucket_idx", bucketIdx),
		)

		// CHG 2026-02-24: 变更=回滚落到分桶表; 之前=单行 product 表回滚; 原因=保持与扣减一致的分桶路由。
		_, err := tx.Exec(
			"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) "+
				"ON DUPLICATE KEY UPDATE stock = stock + VALUES(stock), version = version + 1",
			in.Id, bucketIdx, in.Num,
		)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &product.Empty{}, nil
}
