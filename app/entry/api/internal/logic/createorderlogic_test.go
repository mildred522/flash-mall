package logic

import (
	"context"
	"database/sql"
	"testing"

	"flash-mall/app/entry/api/internal/config"
	"flash-mall/app/entry/api/internal/idgen"
	"flash-mall/app/entry/api/internal/model"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/alicebob/miniredis/v2"
	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"google.golang.org/protobuf/proto"
)

type fixedGenerator struct {
	id string
}

func (g fixedGenerator) NextID() string { return g.id }
func (g fixedGenerator) NodeID() int64  { return 1 }

var _ idgen.Generator = fixedGenerator{}

type stubOrdersModel struct {
	findByRequestID    *model.Orders
	findByRequestIDErr error
}

func (s *stubOrdersModel) Insert(context.Context, *model.Orders) (sql.Result, error) {
	panic("unexpected Insert call")
}

func (s *stubOrdersModel) FindOne(context.Context, string) (*model.Orders, error) {
	panic("unexpected FindOne call")
}

func (s *stubOrdersModel) Update(context.Context, *model.Orders) error {
	panic("unexpected Update call")
}

func (s *stubOrdersModel) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func (s *stubOrdersModel) FindOneByRequestId(context.Context, string) (*model.Orders, error) {
	if s.findByRequestIDErr != nil {
		return nil, s.findByRequestIDErr
	}
	if s.findByRequestID != nil {
		return s.findByRequestID, nil
	}
	return nil, model.ErrNotFound
}

func TestCreateOrderLogic_CreateOrder_RedisLimit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Redis:      rds,
		OrderModel: &stubOrdersModel{findByRequestIDErr: model.ErrNotFound},
		OrderIdGen: fixedGenerator{id: "o-redis"},
	}

	ctx := context.Background()
	l := NewCreateOrderLogic(ctx, svcCtx)

	req := &types.CreateOrderReq{
		UserId:    1,
		ProductId: 1001,
		Amount:    5,
	}

	_, err = l.CreateOrder(req)
	if err == nil {
		t.Error("expected error when DTM server is not configured, got nil")
	}
}

func TestCreateOrderLogic_CreateOrder_ReturnsSnapshotBackedPaymentInfo(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			DtmServer:           "127.0.0.1:36790",
			ProductRpcTarget:    "127.0.0.1:8080",
			OrderRpcTarget:      "127.0.0.1:8090",
			RequestIdTTLSeconds: 60,
			OrderTimeoutSeconds: 60,
		},
		Redis:      rds,
		OrderIdGen: fixedGenerator{id: "o-100"},
		OrderModel: &stubOrdersModel{findByRequestIDErr: model.ErrNotFound},
		SqlConn:    nil,
	}

	l := NewCreateOrderLogic(context.Background(), svcCtx)
	l.genGID = func(string) string { return "gid-test" }

	var capturedCreateOrderReq orderpb.CreateOrderReq
	l.submitSaga = func(saga *dtmgrpc.SagaGrpc) error {
		if len(saga.BinPayloads) != 3 {
			t.Fatalf("expected 3 saga payloads, got %d", len(saga.BinPayloads))
		}
		if err := proto.Unmarshal(saga.BinPayloads[1], &capturedCreateOrderReq); err != nil {
			t.Fatalf("unmarshal create order payload failed: %v", err)
		}
		return nil
	}
	l.loadSnapshotResult = func(orderID string) (*types.CreateOrderResp, error) {
		return &types.CreateOrderResp{
			OrderId:          orderID,
			Status:           "pending_payment",
			PayableAmountFen: 29700,
			PaymentOrderId:   "pay:" + orderID,
		}, nil
	}

	resp, err := l.CreateOrder(&types.CreateOrderReq{
		RequestId:        "req-100",
		UserId:           1,
		ProductId:        100,
		Amount:           3,
		ExpectedPriceFen: 29700,
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	if resp.OrderId != "o-100" || resp.Status != "pending_payment" {
		t.Fatalf("unexpected response envelope: %#v", resp)
	}
	if resp.PayableAmountFen != 29700 || resp.PaymentOrderId != "pay:o-100" {
		t.Fatalf("unexpected payment response: %#v", resp)
	}
	if capturedCreateOrderReq.ExpectedPriceFen != 29700 {
		t.Fatalf("expected expected_price_fen to propagate, got %d", capturedCreateOrderReq.ExpectedPriceFen)
	}
	if capturedCreateOrderReq.OrderId != "o-100" || capturedCreateOrderReq.RequestId != "req-100" {
		t.Fatalf("unexpected create order payload: order_id=%q request_id=%q", capturedCreateOrderReq.OrderId, capturedCreateOrderReq.RequestId)
	}
}

func TestCreateOrderLogic_CreateOrder_RequestIDHitLoadsSnapshotFromDB(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			RequestIdTTLSeconds: 60,
			CacheConf:           cache.CacheConf{},
		},
		Redis: rds,
		OrderModel: &stubOrdersModel{
			findByRequestID: &model.Orders{Id: "o-hit"},
		},
	}

	l := NewCreateOrderLogic(context.Background(), svcCtx)
	l.loadSnapshotResult = func(orderID string) (*types.CreateOrderResp, error) {
		return &types.CreateOrderResp{
			OrderId:          orderID,
			Status:           "pending_payment",
			PayableAmountFen: 9900,
			PaymentOrderId:   "pay:" + orderID,
		}, nil
	}

	resp, err := l.CreateOrder(&types.CreateOrderReq{
		RequestId: "req-hit",
		UserId:    1,
		ProductId: 100,
		Amount:    1,
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.OrderId != "o-hit" || resp.PaymentOrderId != "pay:o-hit" {
		t.Fatalf("unexpected hit response: %#v", resp)
	}
}
