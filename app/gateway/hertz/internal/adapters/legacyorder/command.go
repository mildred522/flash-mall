package legacyorder

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/ports"
)

type Service struct {
	db       *sql.DB
	releaser ports.InventoryReservationReleaser
}

var _ ports.OrderCommands = (*Service)(nil)

func New(db *sql.DB, releaser ports.InventoryReservationReleaser) *Service {
	return &Service{db: db, releaser: releaser}
}

func (s *Service) CancelUser(ctx context.Context, command ports.CancelUserOrderCommand) error {
	return s.closePending(ctx, closePendingCommand{
		orderID: command.OrderID, reason: command.Reason, operatorID: command.UserID,
		ownerColumn: "user_id", ownerID: command.UserID, remarkPrefix: "user cancelled: ", meta: command.Meta,
	})
}

func (s *Service) CloseAdmin(ctx context.Context, command ports.CloseAdminOrderCommand) error {
	return s.closePending(ctx, closePendingCommand{
		orderID: command.OrderID, reason: command.Reason, operatorID: command.OperatorID,
		remarkPrefix: "admin closed: ", meta: command.Meta,
	})
}

type closePendingCommand struct {
	orderID      string
	reason       string
	operatorID   int64
	ownerColumn  string
	ownerID      int64
	remarkPrefix string
	meta         ports.RequestMeta
}

func (s *Service) closePending(ctx context.Context, command closePendingCommand) error {
	if s.db == nil {
		return apperror.New(apperror.CodeInternal, "order datasource is not configured")
	}
	if s.releaser == nil {
		return apperror.New(apperror.CodeInternal, "inventory kitex client is not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := "SELECT status FROM orders WHERE id = ? FOR UPDATE"
	args := []any{command.orderID}
	if command.ownerColumn != "" {
		query = "SELECT status FROM orders WHERE id = ? AND " + command.ownerColumn + " = ? FOR UPDATE"
		args = append(args, command.ownerID)
	}
	var currentStatus int64
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return err
	}
	if !orderstatus.CanPay(currentStatus) {
		return apperror.New(apperror.CodeOrderStatusInvalid, "order cannot be closed")
	}
	result, err := tx.ExecContext(ctx, "UPDATE orders SET status = ? WHERE id = ? AND status = ?", orderstatus.Closed, command.orderID, orderstatus.PendingPayment)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperror.New(apperror.CodeOrderStatusInvalid, "order status changed concurrently")
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
		command.orderID, orderstatus.PendingPayment, orderstatus.Closed, command.operatorID, command.remarkPrefix+command.reason,
	); err != nil {
		return err
	}
	if err = s.releaser.ReleaseStock(ctx, command.orderID, command.reason, command.meta); err != nil {
		return apperror.Wrap(apperror.CodeStockReconcileFailed, "release order stock failed", err)
	}
	return tx.Commit()
}

func (s *Service) ShipAdmin(ctx context.Context, command ports.ShipAdminOrderCommand) error {
	return s.ship(ctx, command.OrderID, 0, command.OperatorID, "admin shipped")
}

func (s *Service) ShipMerchant(ctx context.Context, command ports.ShipMerchantOrderCommand) error {
	return s.ship(ctx, command.OrderID, command.MerchantID, command.MerchantID, "merchant ship order")
}

func (s *Service) ship(ctx context.Context, orderID string, merchantID int64, operatorID int64, remark string) error {
	if s.db == nil {
		return apperror.New(apperror.CodeInternal, "order datasource is not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	query := "SELECT status FROM orders WHERE id = ? FOR UPDATE"
	args := []any{orderID}
	if merchantID > 0 {
		query = "SELECT status FROM orders WHERE id = ? AND merchant_id = ? FOR UPDATE"
		args = append(args, merchantID)
	}
	var currentStatus int64
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return err
	}
	if !orderstatus.CanShip(currentStatus) {
		return apperror.New(apperror.CodeOrderStatusInvalid, "order is not in paid status")
	}
	update := "UPDATE orders SET status = ?, shipped_at = NOW() WHERE id = ? AND status = ?"
	updateArgs := []any{orderstatus.Shipped, orderID, orderstatus.Paid}
	if merchantID > 0 {
		update = "UPDATE orders SET status = ?, shipped_at = NOW() WHERE id = ? AND merchant_id = ? AND status = ?"
		updateArgs = []any{orderstatus.Shipped, orderID, merchantID, orderstatus.Paid}
	}
	result, err := tx.ExecContext(ctx, update, updateArgs...)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperror.New(apperror.CodeOrderStatusInvalid, "order status changed concurrently")
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, ?)",
		orderID, orderstatus.Paid, orderstatus.Shipped, operatorID, remark,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ConfirmReceipt(ctx context.Context, command ports.ConfirmReceiptCommand) error {
	if s.db == nil {
		return apperror.New(apperror.CodeInternal, "order datasource is not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var currentStatus int64
	if err = tx.QueryRowContext(ctx, "SELECT status FROM orders WHERE id = ? AND user_id = ? FOR UPDATE", command.OrderID, command.UserID).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.CodeOrderNotFound, "order not found")
		}
		return err
	}
	if !orderstatus.CanConfirmReceipt(currentStatus) {
		return apperror.New(apperror.CodeOrderStatusInvalid, "order is not in shipped status")
	}
	result, err := tx.ExecContext(ctx, "UPDATE orders SET status = ?, completed_at = NOW() WHERE id = ? AND status = ?", orderstatus.Completed, command.OrderID, orderstatus.Shipped)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperror.New(apperror.CodeOrderStatusInvalid, "order status changed concurrently")
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, ?, 'buyer confirmed receipt')",
		command.OrderID, orderstatus.Shipped, orderstatus.Completed, command.UserID,
	); err != nil {
		return err
	}
	return tx.Commit()
}
