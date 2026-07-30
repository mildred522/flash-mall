package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/dtm-labs/dtm/client/dtmcli/dtmimp"

	"flash-mall/app/common/observability"
	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/job"
	"flash-mall/app/order/rpc/internal/logic"
	"flash-mall/app/order/rpc/internal/server"
	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

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
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	shutdownTracing, err := observability.SetupTracing(context.Background(), c.Observability.Tracing)
	if err != nil {
		panic(err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	ctx := svc.NewServiceContext(c)
	job.NewOutboxPublisher(ctx).Start()
	job.NewOrderPaidProjectionConsumer(ctx).Start()
	job.NewPaymentRecovery(ctx).OnProviderPaid(func(callCtx context.Context, payment job.ProviderPaidPayment) error {
		callbackBody, err := json.Marshal(map[string]any{
			"trade_status": "SUCCESS", "provider": "alipay_sandbox",
			"event_id": "reconcile:" + payment.OutTradeNo, "paid_amount_fen": payment.AmountFen,
			"provider_trade_no": payment.TradeNo,
		})
		if err != nil {
			return err
		}
		_, err = logic.NewMarkOrderPaidLogic(callCtx, ctx).MarkPaid(&order.MarkOrderPaidReq{
			OrderId: payment.OrderID, PaymentOrderId: payment.PaymentOrderID,
			OutTradeNo: payment.OutTradeNo, CallbackBody: string(callbackBody),
		})
		return err
	}).Start()

	observability.StartDiagnostics(c.MetricsAddr, c.PprofAddr)

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
