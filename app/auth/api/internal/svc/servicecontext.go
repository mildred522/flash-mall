package svc

import (
	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/sessionstate"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config
	Store  authstore.AuthStore
}

func NewServiceContext(c config.Config) *ServiceContext {
	var stateStore sessionstate.StateStore
	if c.RedisConf.Host != "" {
		stateStore = sessionstate.NewRedisStateStore(redis.MustNewRedis(c.RedisConf))
	}
	if c.DataSource != "" {
		return NewServiceContextWithStore(c, authstore.NewSQLStore(sqlx.NewMysql(c.DataSource), c.DemoPassword, stateStore))
	}
	return NewServiceContextWithStore(c, authstore.NewStoreWithState(c.DemoPassword, stateStore))
}

func NewServiceContextWithStore(c config.Config, store authstore.AuthStore) *ServiceContext {
	if c.RefreshTokenTTLSeconds <= 0 {
		c.RefreshTokenTTLSeconds = 7 * 24 * 60 * 60
	}
	if c.CodeTTLSeconds <= 0 {
		c.CodeTTLSeconds = 5 * 60
	}
	if c.RefreshCookieName == "" {
		c.RefreshCookieName = "fm_refresh_token"
	}

	return &ServiceContext{
		Config: c,
		Store:  store,
	}
}
