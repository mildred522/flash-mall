package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetOrderDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type orderDetailRow struct {
	OrderID            string `db:"order_id"`
	UserID             int64  `db:"user_id"`
	OrderStatus        int64  `db:"order_status"`
	Amount             int64  `db:"amount"`
	OriginUnitPriceFen int64  `db:"origin_unit_price_fen"`
	PayableAmountFen   int64  `db:"payable_amount_fen"`
	DiscountAmountFen  int64  `db:"discount_amount_fen"`
	PaymentStatus      int64  `db:"payment_status"`
}

func NewGetOrderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderDetailLogic {
	return &GetOrderDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetOrderDetailLogic) GetOrderDetail(in *order.GetOrderDetailReq) (*order.GetOrderDetailResp, error) {
	orderID := strings.TrimSpace(in.OrderId)
	if orderID == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	var row orderDetailRow
	err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &row, `
SELECT o.id AS order_id,
       o.user_id AS user_id,
       o.status AS order_status,
       s.amount AS amount,
       s.origin_unit_price_fen AS origin_unit_price_fen,
       s.payable_amount_fen AS payable_amount_fen,
       s.discount_amount_fen AS discount_amount_fen,
       p.status AS payment_status
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ?
LIMIT 1`, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "order detail not found")
		}
		return nil, err
	}

	return &order.GetOrderDetailResp{
		OrderId:           row.OrderID,
		UserId:            row.UserID,
		OrderStatus:       orderDetailStatusText(row.OrderStatus),
		PayableAmountFen:  row.PayableAmountFen,
		PaymentStatus:     paymentStatusText(row.PaymentStatus),
		OriginPriceFen:    row.OriginUnitPriceFen * row.Amount,
		DiscountAmountFen: row.DiscountAmountFen,
	}, nil
}

func orderDetailStatusText(statusCode int64) string {
	switch statusCode {
	case 0:
		return "PENDING_PAYMENT"
	case 1:
		return "PAID"
	case 2:
		return "CLOSED"
	case 3:
		return "SHIPPED"
	case 4:
		return "COMPLETED"
	case 5:
		return "REFUND_REQUESTED"
	case 6:
		return "REFUNDED"
	case 7:
		return "REFUND_FAILED"
	default:
		return "UNKNOWN"
	}
}

func paymentStatusText(statusCode int64) string {
	switch statusCode {
	case 0:
		return "INIT"
	case 1:
		return "SUCCESS"
	case 2:
		return "FAILED"
	case 3:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}
