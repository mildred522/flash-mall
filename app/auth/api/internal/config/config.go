package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	JwtAuthSecret               string
	JwtExpireSeconds            int64
	DemoPassword                string
	DataSource                  string
	RedisConf                   redis.RedisConf
	RefreshTokenTTLSeconds      int64
	CodeTTLSeconds              int64
	RefreshCookieName           string
	LoginFailWindowSeconds      int64
	LoginFailPhoneMaxAttempts   int64
	LoginFailIPMaxAttempts      int64
	CodeSendCooldownSeconds     int64
	CodeSendPhoneWindowSeconds  int64
	CodeSendPhoneMaxAttempts    int64
	CodeSendIPWindowSeconds     int64
	CodeSendIPMaxAttempts       int64
	VerificationCodeMaxAttempts int64
	SecurityAuditRecentLimit    int64
	SecurityAuditRetentionLimit int64
}
