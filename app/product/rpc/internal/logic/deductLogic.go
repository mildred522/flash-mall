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

	// 3) 在 Barrier 保护下执行本地事务
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		l.Infow("deduct stock start",
			logx.Field("product_id", in.Id),
			logx.Field("amount", in.Num),
			logx.Field("bucket_idx", bucketIdx),
		)

		// CHG 2026-02-24: 变更=扣减落到分桶表; 之前=单行 product 表扣减; 原因=降低热点行冲突。
		res, err := tx.Exec(
			"UPDATE product_stock_bucket SET stock = stock - ?, version = version + 1 WHERE product_id = ? AND bucket_idx = ? AND stock >= ?",
			in.Num, in.Id, bucketIdx, in.Num,
		)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected > 0 {
			return nil
		}

		// 0 行受影响：可能是分桶不存在或库存不足
		var exists int
		err = tx.QueryRow("SELECT 1 FROM product_stock_bucket WHERE product_id = ? AND bucket_idx = ?", in.Id, bucketIdx).Scan(&exists)
		if err != nil {
			if err == sql.ErrNoRows {
				return status.Error(codes.NotFound, "库存分桶不存在")
			}
			return err
		}
		return status.Error(codes.Aborted, "库存不足")
	})
	if err != nil {
		return nil, err
	}

	return &product.Empty{}, nil
}
