package middleware

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
	"golang.org/x/time/rate"
)

// NewRateLimitMiddleware 基于令牌桶的限流中间件（超限直接返回 429）
func NewRateLimitMiddleware(limiter *rate.Limiter) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || limiter.Allow() {
				next(w, r)
				return
			}
			// CHG 2026-02-24: 变更=限流失败直接返回 429; 之前=请求继续进入后端; 原因=保护核心链路免于雪崩。
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":429,"message":"rate limit exceeded"}`))
		}
	}
}
