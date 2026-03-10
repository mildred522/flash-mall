package logic

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
)

type SystemHealthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSystemHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemHealthLogic {
	return &SystemHealthLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SystemHealthLogic) SystemHealth() (*types.SystemHealthResp, error) {
	deps := make([]types.DependencyStatus, 0, 6)
	overall := true

	appendDep := func(name string, ok bool, detail string) {
		if !ok {
			overall = false
		}
		deps = append(deps, types.DependencyStatus{
			Name:   name,
			Ok:     ok,
			Detail: detail,
		})
	}

	mysqlOK, mysqlDetail := l.checkMySQL()
	appendDep("mysql", mysqlOK, mysqlDetail)
	redisOK, redisDetail := l.checkRedis()
	appendDep("redis", redisOK, redisDetail)
	dtmOK, dtmDetail := l.checkTCP(strings.TrimSpace(l.svcCtx.Config.DtmServer))
	appendDep("dtm", dtmOK, dtmDetail)
	orderRPCOK, orderRPCDetail := l.checkTCP(strings.TrimSpace(l.svcCtx.Config.OrderRpcTarget))
	appendDep("order-rpc", orderRPCOK, orderRPCDetail)
	productRPCOK, productRPCDetail := l.checkTCP(strings.TrimSpace(l.svcCtx.Config.ProductRpcTarget))
	appendDep("product-rpc", productRPCOK, productRPCDetail)
	rabbitOK, rabbitDetail := l.checkRabbit()
	appendDep("rabbitmq", rabbitOK, rabbitDetail)

	return &types.SystemHealthResp{
		Overall:      overall,
		ServerTime:   time.Now().Unix(),
		Dependencies: deps,
	}, nil
}

func (l *SystemHealthLogic) checkMySQL() (bool, string) {
	rawDB, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return false, fmt.Sprintf("raw db error: %v", err)
	}
	ctx, cancel := context.WithTimeout(l.ctx, 1500*time.Millisecond)
	defer cancel()
	if pingErr := rawDB.PingContext(ctx); pingErr != nil {
		return false, pingErr.Error()
	}
	return true, "ping ok"
}

func (l *SystemHealthLogic) checkRedis() (bool, string) {
	key := fmt.Sprintf("health:ui:%d", time.Now().UnixNano())
	if err := l.svcCtx.Redis.SetexCtx(l.ctx, key, "ok", 5); err != nil {
		return false, fmt.Sprintf("set error: %v", err)
	}
	val, err := l.svcCtx.Redis.GetCtx(l.ctx, key)
	_, _ = l.svcCtx.Redis.DelCtx(l.ctx, key)
	if err != nil {
		return false, fmt.Sprintf("get error: %v", err)
	}
	if val != "ok" {
		return false, fmt.Sprintf("unexpected value: %s", val)
	}
	return true, "set/get ok"
}

func (l *SystemHealthLogic) checkTCP(addr string) (bool, string) {
	if addr == "" {
		return false, "empty address"
	}
	conn, err := net.DialTimeout("tcp", addr, 1500*time.Millisecond)
	if err != nil {
		return false, err.Error()
	}
	_ = conn.Close()
	return true, "tcp connect ok"
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
