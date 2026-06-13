// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"

	"flash-mall/app/common/observability"
	"flash-mall/app/order/api/internal/config"
	"flash-mall/app/order/api/internal/handler"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/job"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/order-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	shutdownTracing, err := observability.SetupTracing(context.Background(), c.Observability.Tracing)
	if err != nil {
		panic(err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	observability.StartDiagnostics(c.MetricsAddr, c.PprofAddr)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	job.NewCloseOrderJob(ctx).Start()
	job.NewOrderEventConsumer(ctx).Start()
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
