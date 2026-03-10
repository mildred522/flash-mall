package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/dtm-labs/dtm/client/dtmcli/dtmimp"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/job"
	"flash-mall/app/order/rpc/internal/server"
	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/order.yaml", "the config file")

func main() {
	flag.Parse()

	dtmimp.BarrierTableName = "barrier"

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)
	job.NewOutboxPublisher(ctx).Start()

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

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		order.RegisterOrderServer(grpcServer, server.NewOrderServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
