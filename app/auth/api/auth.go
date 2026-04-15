package main

import (
	"flag"
	"fmt"

	"flash-mall/app/auth/api/internal/config"
	"flash-mall/app/auth/api/internal/handler"
	"flash-mall/app/auth/api/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/auth-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting auth server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
