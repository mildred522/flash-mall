// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"flash-mall/app/order/api/internal/config"
	"flash-mall/app/order/api/internal/handler"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/job"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/order-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	if c.MetricsAddr != "" {
		http.Handle("/metrics", promhttp.Handler())
		go func() {
			_ = http.ListenAndServe(c.MetricsAddr, nil)
		}()
	}
	if c.PprofAddr != "" {
		go func() {
			_ = http.ListenAndServe(c.PprofAddr, nil)
		}()
	}

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	job.NewCloseOrderJob(ctx).Start()
	job.NewOrderEventConsumer(ctx).Start()
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
