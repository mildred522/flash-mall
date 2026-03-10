package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrdersModel = (*customOrdersModel)(nil)

type (
	// OrdersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOrdersModel.
	OrdersModel interface {
		ordersModel
		FindOneByRequestId(ctx context.Context, requestId string) (*Orders, error)
	}

	customOrdersModel struct {
		*defaultOrdersModel
	}
)

// NewOrdersModel returns a model for the database table.
func NewOrdersModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrdersModel {
	return &customOrdersModel{
		defaultOrdersModel: newOrdersModel(conn, c, opts...),
	}
}

// FindOneByRequestId 根据 request_id 查询订单（幂等查询）。
func (m *customOrdersModel) FindOneByRequestId(ctx context.Context, requestId string) (*Orders, error) {
	// CHG 2026-02-07: 变更=新增 request_id 查询; 之前=仅主键查询; 原因=重复请求复用同一订单。
	if requestId == "" {
		return nil, ErrNotFound
	}
	cacheKey := fmt.Sprintf("%s%v", cacheOrdersRequestIdPrefix, requestId)
	var resp Orders
	err := m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query := fmt.Sprintf("select %s from %s where `request_id` = ? limit 1", ordersRows, m.table)
		return conn.QueryRowCtx(ctx, v, query, requestId)
	})
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
