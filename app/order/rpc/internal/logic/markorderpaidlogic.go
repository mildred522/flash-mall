package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"flash-mall/app/order/rpc/internal/svc"
	order "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MarkOrderPaidLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type paymentCallbackPayload struct {
	TradeStatus   string `json:"trade_status"`
	Provider      string `json:"provider"`
	EventID       string `json:"event_id"`
	PaidAmountFen int64  `json:"paid_amount_fen"`
}

func NewMarkOrderPaidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkOrderPaidLogic {
	return &MarkOrderPaidLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkOrderPaidLogic) MarkPaid(in *order.MarkOrderPaidReq) (*order.MarkOrderPaidResp, error) {
	if in.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if in.PaymentOrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_order_id is required")
	}
	if in.OutTradeNo == "" {
		return nil, status.Error(codes.InvalidArgument, "out_trade_no is required")
	}
	callback, err := parsePaymentCallbackPayload(in.CallbackBody)
	if err != nil {
		return nil, err
	}

	db, err := l.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db connection failed")
	}

	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "begin tx failed")
	}
	defer func() { _ = tx.Rollback() }()

	var orderStatus, paymentStatus, payableAmountFen int64
	err = tx.QueryRowContext(l.ctx, `
SELECT o.status, p.status, p.payable_amount_fen
FROM payment_order p
JOIN orders o ON o.id = p.order_id
WHERE p.id = ? AND p.out_trade_no = ? AND p.order_id = ?
FOR UPDATE`, in.PaymentOrderId, in.OutTradeNo, in.OrderId).Scan(&orderStatus, &paymentStatus, &payableAmountFen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "payment order not found")
		}
		return nil, err
	}

	if callback.PaidAmountFen != payableAmountFen {
		if err := insertPaymentCallbackEvent(l.ctx, tx, in, callback, "FAILED_AMOUNT_MISMATCH", "paid amount does not match payable amount"); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, status.Error(codes.FailedPrecondition, "paid amount does not match payable amount")
	}

	if orderStatus == 1 || paymentStatus == 1 {
		if err := insertPaymentCallbackEvent(l.ctx, tx, in, callback, "IDEMPOTENT", ""); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &order.MarkOrderPaidResp{Updated: false, OrderStatus: "PAID"}, nil
	}
	if orderStatus == 2 {
		if err := insertPaymentCallbackEvent(l.ctx, tx, in, callback, "CLOSED", ""); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &order.MarkOrderPaidResp{Updated: false, OrderStatus: "CLOSED"}, nil
	}

	orderResult, err := tx.ExecContext(l.ctx, "UPDATE orders SET status = 1, update_time = NOW() WHERE id = ? AND status = 0", in.OrderId)
	if err != nil {
		return nil, err
	}
	orderRows, err := orderResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if orderRows == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &order.MarkOrderPaidResp{Updated: false, OrderStatus: "UNKNOWN"}, nil
	}

	paymentResult, err := tx.ExecContext(l.ctx, "UPDATE payment_order SET status = 1, paid_at = NOW(), callback_payload = CAST(? AS JSON), update_time = NOW() WHERE id = ? AND out_trade_no = ? AND order_id = ? AND status = 0", in.CallbackBody, in.PaymentOrderId, in.OutTradeNo, in.OrderId)
	if err != nil {
		return nil, err
	}
	paymentRows, err := paymentResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if paymentRows == 0 {
		return nil, status.Error(codes.FailedPrecondition, "payment order is not payable")
	}

	if err := insertPaymentCallbackEvent(l.ctx, tx, in, callback, "SUCCESS", ""); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &order.MarkOrderPaidResp{Updated: true, OrderStatus: "PAID"}, nil
}

func parsePaymentCallbackPayload(body string) (paymentCallbackPayload, error) {
	if body == "" {
		return paymentCallbackPayload{}, status.Error(codes.InvalidArgument, "callback_body is required")
	}

	var payload paymentCallbackPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return paymentCallbackPayload{}, status.Error(codes.InvalidArgument, "callback_body is invalid")
	}
	if payload.TradeStatus != "" && payload.TradeStatus != "SUCCESS" {
		return paymentCallbackPayload{}, status.Error(codes.FailedPrecondition, "payment callback is not successful")
	}
	if payload.PaidAmountFen <= 0 {
		return paymentCallbackPayload{}, status.Error(codes.InvalidArgument, "paid_amount_fen is required")
	}
	if payload.Provider == "" {
		payload.Provider = "mock"
	}
	return payload, nil
}

func insertPaymentCallbackEvent(ctx context.Context, tx *sql.Tx, in *order.MarkOrderPaidReq, payload paymentCallbackPayload, processStatus, errorMessage string) error {
	eventID := payload.EventID
	if eventID == "" {
		eventID = fmt.Sprintf("%s:%s:%s", payload.Provider, in.PaymentOrderId, in.OutTradeNo)
	}

	_, err := tx.ExecContext(ctx, `
INSERT IGNORE INTO payment_callback_event (
  provider, event_id, payment_order_id, order_id, out_trade_no,
  paid_amount_fen, signature_valid, process_status, error_message, raw_payload
	) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, CAST(? AS JSON))`,
		payload.Provider,
		eventID,
		in.PaymentOrderId,
		in.OrderId,
		in.OutTradeNo,
		payload.PaidAmountFen,
		processStatus,
		errorMessage,
		in.CallbackBody,
	)
	return err
}
