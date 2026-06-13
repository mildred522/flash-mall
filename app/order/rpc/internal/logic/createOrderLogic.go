package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	commonobs "flash-mall/app/common/observability"
	"flash-mall/app/order/rpc/internal/job"
	"flash-mall/app/order/rpc/internal/pricing"
	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const orderStatusPendingPayment = "pending_payment"

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrder is the order creation saga branch in order-rpc.
func (l *CreateOrderLogic) CreateOrder(in *order.CreateOrderReq) (*order.CreateOrderResp, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if in.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}
	if in.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if l.svcCtx.ProductRpc == nil {
		return nil, status.Error(codes.Internal, "product rpc not configured")
	}

	requestID := in.RequestId
	if requestID == "" {
		requestID = in.OrderId
	}
	paymentOrderID := paymentOrderIDFor(in.OrderId)

	span := trace.SpanFromContext(l.ctx)
	span.SetAttributes(
		attribute.String("order.id", in.GetOrderId()),
		attribute.String("order.request_id", requestID),
		attribute.Int64("user.id", in.GetUserId()),
		attribute.Int64("product.id", in.GetProductId()),
		attribute.Int64("order.amount", in.GetAmount()),
	)
	l.Infow("order rpc create order", commonobs.OrderFields(l.ctx, in.GetOrderId(), requestID)...)

	card, err := l.svcCtx.ProductRpc.GetProductCard(l.ctx, &productclient.GetProductCardReq{
		ProductId: in.ProductId,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "load product card failed")
	}

	quote, err := pricing.BuildQuote(card, in.Amount)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	if in.ExpectedPriceFen > 0 && in.ExpectedPriceFen != quote.PayableAmountFen {
		return nil, status.Error(codes.FailedPrecondition, "price changed, please retry checkout")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "DTM barrier not found")
	}

	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}

	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		if _, execErr := tx.Exec(
			"INSERT IGNORE INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, ?, ?, ?, 0)",
			in.OrderId, requestID, in.UserId, in.ProductId, in.Amount,
		); execErr != nil {
			return execErr
		}
		if _, execErr := tx.Exec(`
INSERT IGNORE INTO order_price_snapshot (
	order_id,
	product_id,
	supplier_id,
	product_name,
	amount,
	origin_unit_price_fen,
	sale_unit_price_fen,
	payable_amount_fen,
	discount_amount_fen,
	promotion_type,
	promotion_tag
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.OrderId,
			quote.ProductId,
			quote.SupplierId,
			quote.ProductName,
			quote.Amount,
			quote.OriginUnitPriceFen,
			quote.SaleUnitPriceFen,
			quote.PayableAmountFen,
			quote.DiscountAmountFen,
			quote.PromotionType,
			quote.PromotionTag,
		); execErr != nil {
			return execErr
		}
		if _, execErr := tx.Exec(`
INSERT IGNORE INTO payment_order (
	id,
	order_id,
	user_id,
	payable_amount_fen,
	status,
	out_trade_no
) VALUES (?, ?, ?, ?, 0, ?)`,
			paymentOrderID,
			in.OrderId,
			in.UserId,
			quote.PayableAmountFen,
			outTradeNoFor(in.OrderId),
		); execErr != nil {
			return execErr
		}

		payload, marshalErr := json.Marshal(map[string]any{
			"event_id":           "order.created:" + in.OrderId,
			"event_type":         "order.created",
			"order_id":           in.OrderId,
			"request_id":         requestID,
			"user_id":            in.UserId,
			"product_id":         in.ProductId,
			"amount":             in.Amount,
			"payable_amount_fen": quote.PayableAmountFen,
			"payment_order_id":   paymentOrderID,
			"created_at":         time.Now().Unix(),
		})
		if marshalErr != nil {
			return marshalErr
		}

		return job.InsertOrderCreatedOutbox(tx, in.OrderId, string(payload))
	})
	if err != nil {
		return nil, err
	}

	return &order.CreateOrderResp{
		OrderId:          in.OrderId,
		Status:           orderStatusPendingPayment,
		PayableAmountFen: quote.PayableAmountFen,
		PaymentOrderId:   paymentOrderID,
	}, nil
}

func paymentOrderIDFor(orderID string) string {
	return fmt.Sprintf("pay:%s", orderID)
}

func outTradeNoFor(orderID string) string {
	return fmt.Sprintf("mock-%s", orderID)
}
