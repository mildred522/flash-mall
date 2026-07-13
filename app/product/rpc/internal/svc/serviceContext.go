package svc

import (
	"flash-mall/app/product/rpc/internal/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config  config.Config
	SqlConn sqlx.SqlConn
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:  c,
		SqlConn: sqlx.NewMysql(c.DataSource),
	}
}
