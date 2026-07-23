package svc

import (
	"strings"

	"flash-mall/app/auth/api/internal/audit"
	"flash-mall/app/auth/api/internal/authstore"
	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/risk"
	"flash-mall/app/auth/api/internal/sessionstate"
	"flash-mall/app/common/mysqlguard"

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
	switch normalizeStorageMode(c) {
	case "mysql":
		if strings.TrimSpace(c.DataSource) == "" {
			panic("auth storage mode mysql requires DataSource")
		}
		mysqlguard.MustUTF8MB4("auth", c.DataSource)
		return newServiceContext(c, authstore.NewSQLStore(sqlx.NewMysql(c.DataSource), c.DemoPassword, stateStore), limiter, recorder)
	case "memory":
		return newServiceContext(c, authstore.NewStoreWithState(c.DemoPassword, stateStore), limiter, recorder)
	default:
		panic("unreachable auth storage mode")
	}
}

func normalizeStorageMode(c config.Config) string {
	mode := strings.ToLower(strings.TrimSpace(c.StorageMode))
	if mode == "" && strings.TrimSpace(c.Name) == "" {
		return "memory"
	}
	if mode != "mysql" && mode != "memory" {
		panic("auth StorageMode must be explicitly set to mysql or memory")
	}
	return mode
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
	if c.SecurityAuditRetentionLimit <= 0 {
		c.SecurityAuditRetentionLimit = int64(auditRetentionLimit(c))
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
	retentionLimit := auditRetentionLimit(c)
	if c.RedisConf.Host == "" {
		return nil, risk.NewMemoryLimiter(), audit.NewMemoryRecorder(retentionLimit)
	}
	rds := redis.MustNewRedis(c.RedisConf)
	return sessionstate.NewRedisStateStore(rds), risk.NewRedisLimiter(rds), audit.NewRedisRecorder(rds, retentionLimit)
}

func auditRetentionLimit(c config.Config) int {
	limit := int(c.SecurityAuditRetentionLimit)
	if limit <= 0 {
		limit = 200
	}
	recentLimit := int(c.SecurityAuditRecentLimit)
	if recentLimit > limit {
		limit = recentLimit
	}
	return limit
}
