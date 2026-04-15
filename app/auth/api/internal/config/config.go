package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	JwtAuthSecret          string
	JwtExpireSeconds       int64
	DemoPassword           string
	DataSource             string
	RedisConf              redis.RedisConf
	RefreshTokenTTLSeconds int64
	CodeTTLSeconds         int64
	RefreshCookieName      string
}
