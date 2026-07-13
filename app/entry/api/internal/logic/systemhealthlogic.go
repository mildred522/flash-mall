package logic

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"strings"
	"time"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"
)

type SystemHealthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

const dependencyCheckTimeout = time.Second

func NewSystemHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemHealthLogic {
	return &SystemHealthLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SystemHealthLogic) SystemHealth() (*types.SystemHealthResp, error) {
	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{name: "mysql", fn: l.checkMySQL},
		{name: "redis", fn: l.checkRedis},
		{name: "dtm", fn: func() (bool, string) { return l.checkTCP(strings.TrimSpace(l.svcCtx.Config.DtmServer)) }},
		{name: "order-rpc", fn: func() (bool, string) { return l.checkTCP(strings.TrimSpace(l.svcCtx.Config.OrderRpcTarget)) }},
		{name: "product-rpc", fn: func() (bool, string) { return l.checkTCP(strings.TrimSpace(l.svcCtx.Config.ProductRpcTarget)) }},
		{name: "rabbitmq", fn: l.checkRabbit},
	}

	type result struct {
		dep types.DependencyStatus
	}
	resultCh := make(chan result, len(checks))
	for _, check := range checks {
		check := check
		go func() {
			ok, detail := check.fn()
			resultCh <- result{dep: types.DependencyStatus{Name: check.name, Ok: ok, Detail: detail}}
		}()
	}

	deps := make([]types.DependencyStatus, 0, len(checks))
	seen := make(map[string]bool, len(checks))
	deadline := time.After(1500 * time.Millisecond)
	for len(deps) < len(checks) {
		select {
		case res := <-resultCh:
			if seen[res.dep.Name] {
				continue
			}
			seen[res.dep.Name] = true
			deps = append(deps, res.dep)
		case <-deadline:
			for _, check := range checks {
				if !seen[check.name] {
					deps = append(deps, types.DependencyStatus{Name: check.name, Ok: false, Detail: "health check timed out"})
				}
			}
		}
	}

	overall := true
	for _, dep := range deps {
		if !dep.Ok {
			overall = false
			break
		}
	}

	uptime := time.Since(l.svcCtx.StartTime).Truncate(time.Second).String()

	return &types.SystemHealthResp{
		Overall:      overall,
		Version:      "dev",
		Uptime:       uptime,
		Goroutines:   runtime.NumGoroutine(),
		ServerTime:   time.Now().Unix(),
		Dependencies: deps,
	}, nil
}

func (l *SystemHealthLogic) checkMySQL() (bool, string) {
	addr := mysqlAddrFromDSN(l.svcCtx.Config.DataSource)
	if addr == "" {
		return false, "mysql address not found in datasource"
	}
	return l.checkTCP(addr)
}

func (l *SystemHealthLogic) checkRedis() (bool, string) {
	return l.checkTCP(strings.TrimSpace(l.svcCtx.Config.RedisConf.Host))
}

func (l *SystemHealthLogic) checkTCP(addr string) (bool, string) {
	if addr == "" {
		return false, "empty address"
	}
	conn, err := net.DialTimeout("tcp", addr, dependencyCheckTimeout)
	if err == nil {
		_ = conn.Close()
		return true, "tcp connect ok"
	}

	if fallback, ok := localhostFallbackForDockerHost(addr); ok {
		fallbackConn, fallbackErr := net.DialTimeout("tcp", fallback, dependencyCheckTimeout)
		if fallbackErr == nil {
			_ = fallbackConn.Close()
			return true, fmt.Sprintf("tcp connect ok via %s fallback", fallback)
		}
	}

	return false, err.Error()
}

func localhostFallbackForDockerHost(addr string) (string, bool) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(host, "host.docker.internal") {
		return "", false
	}
	return net.JoinHostPort("127.0.0.1", port), true
}

func mysqlAddrFromDSN(dsn string) string {
	start := strings.Index(dsn, "@tcp(")
	if start < 0 {
		return ""
	}
	start += len("@tcp(")
	end := strings.Index(dsn[start:], ")")
	if end < 0 {
		return ""
	}
	return dsn[start : start+end]
}

func (l *SystemHealthLogic) checkRabbit() (bool, string) {
	rawURL := strings.TrimSpace(l.svcCtx.Config.RabbitMQURL)
	if rawURL == "" {
		return false, "empty rabbitmq url"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Sprintf("parse url error: %v", err)
	}
	if u.Host == "" {
		return false, "missing rabbitmq host"
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":5672"
	}
	return l.checkTCP(host)
}
