package svc

import (
	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/risk"
	"flash-mall/app/auth/api/internal/sessionstate"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config        config.Config
	Store         authstore.AuthStore
	RiskLimiter   risk.Limiter
	AuditRecorder audit.Recorder
}

func NewServiceContext(c config.Config) *ServiceContext {
	stateStore, limiter, recorder := newFoundationDeps(c)
	if c.DataSource != "" {
		return newServiceContext(c, authstore.NewSQLStore(sqlx.NewMysql(c.DataSource), c.DemoPassword, stateStore), limiter, recorder)
	}
	return newServiceContext(c, authstore.NewStoreWithState(c.DemoPassword, stateStore), limiter, recorder)
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
	if c.LoginFailWindowSeconds <= 0 {
		c.LoginFailWindowSeconds = 15 * 60
	}
	if c.LoginFailPhoneMaxAttempts <= 0 {
		c.LoginFailPhoneMaxAttempts = 5
	}
	if c.LoginFailIPMaxAttempts <= 0 {
		c.LoginFailIPMaxAttempts = 20
	}
	if c.CodeSendCooldownSeconds <= 0 {
		c.CodeSendCooldownSeconds = 60
	}
	if c.CodeSendPhoneWindowSeconds <= 0 {
		c.CodeSendPhoneWindowSeconds = 10 * 60
	}
	if c.CodeSendPhoneMaxAttempts <= 0 {
		c.CodeSendPhoneMaxAttempts = 3
	}
	if c.CodeSendIPWindowSeconds <= 0 {
		c.CodeSendIPWindowSeconds = 10 * 60
	}
	if c.CodeSendIPMaxAttempts <= 0 {
		c.CodeSendIPMaxAttempts = 10
	}
	if c.VerificationCodeMaxAttempts <= 0 {
		c.VerificationCodeMaxAttempts = 5
	}
	if c.SecurityAuditRecentLimit <= 0 {
		c.SecurityAuditRecentLimit = 10
	}
	_, limiter, recorder := newFoundationDeps(c)

	return newServiceContext(c, store, limiter, recorder)
}

func newServiceContext(c config.Config, store authstore.AuthStore, limiter risk.Limiter, recorder audit.Recorder) *ServiceContext {
	return &ServiceContext{
		Config:        c,
		Store:         store,
		RiskLimiter:   limiter,
		AuditRecorder: recorder,
	}
}

func newFoundationDeps(c config.Config) (sessionstate.StateStore, risk.Limiter, audit.Recorder) {
	if c.RedisConf.Host == "" {
		return nil, risk.NewMemoryLimiter(), audit.NewMemoryRecorder(int(c.SecurityAuditRecentLimit))
	}
	rds := redis.MustNewRedis(c.RedisConf)
	return sessionstate.NewRedisStateStore(rds), risk.NewRedisLimiter(rds), audit.NewRedisRecorder(rds, int(c.SecurityAuditRecentLimit))
}
