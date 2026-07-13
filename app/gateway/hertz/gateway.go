package main

import (
	"flag"
	"time"

	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/handler"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/zeromicro/go-zero/core/conf"
)

var configFile = flag.String("f", "etc/gateway.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	svcCtx := svc.NewServiceContext(c)
	h := server.Default(server.WithHostPorts(c.NormalizedListenOn()))
	handler.RegisterRoutes(h, svcCtx, time.Now())
	h.Spin()
}
