package repository

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

const (
	reservationLedgerModeOff     = "off"
	reservationLedgerModeShadow  = "shadow"
	reservationLedgerModeEnforce = "enforce"
)

type reservationLedgerRecord struct {
	OrderID    string
	ProductID  int64
	Quantity   int64
	ShardIndex int
	Status     domain.ReservationStatus
	ExpiresAt  time.Time
	RequestID  string
	TraceID    string
}

func (r *RedisMySQLRepository) WithReservationLedgerMode(mode string) *RedisMySQLRepository {
	r.reservationLedgerMode = normalizeReservationLedgerMode(mode)
	return r
}

func (r *RedisMySQLRepository) insertReservationLedger(ctx context.Context, record reservationLedgerRecord) error {
	if r.db == nil || normalizeReservationLedgerMode(r.reservationLedgerMode) == reservationLedgerModeOff {
		return nil
	}
	if err := r.ensureReservationLedgerTable(ctx); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO inventory_reservation
  (order_id, product_id, quantity, shard_index, status, expires_at, version, request_id, trace_id)
VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
ON DUPLICATE KEY UPDATE order_id = VALUES(order_id)
`,
		record.OrderID,
		record.ProductID,
		record.Quantity,
		record.ShardIndex,
		strings.ToUpper(string(record.Status)),
		record.ExpiresAt,
		record.RequestID,
		record.TraceID,
	)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write inventory reservation ledger failed", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read inventory reservation ledger result failed", err)
	}
	if affected == 0 {
		var productID, quantity int64
		if err := r.db.QueryRowContext(ctx, "SELECT product_id, quantity FROM inventory_reservation WHERE order_id = ?", record.OrderID).Scan(&productID, &quantity); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "read inventory reservation ledger identity failed", err)
		}
		if productID != record.ProductID || quantity != record.Quantity {
			return domain.ErrReservationConflict
		}
	}
	return nil
}

func (r *RedisMySQLRepository) recordReservation(ctx context.Context, record reservationLedgerRecord) error {
	err := r.insertReservationLedger(ctx, record)
	if err != nil && normalizeReservationLedgerMode(r.reservationLedgerMode) == reservationLedgerModeShadow {
		log.Printf("inventory reservation ledger shadow write failed: order_id=%s err=%v", record.OrderID, err)
		return nil
	}
	return err
}

func (r *RedisMySQLRepository) transitionReservationLedger(ctx context.Context, orderID string, target domain.ReservationStatus, allowed ...domain.ReservationStatus) error {
	mode := normalizeReservationLedgerMode(r.reservationLedgerMode)
	if r.db == nil || mode == reservationLedgerModeOff || orderID == "" || len(allowed) == 0 {
		return nil
	}
	if err := r.ensureReservationLedgerTable(ctx); err != nil {
		if mode == reservationLedgerModeShadow {
			log.Printf("inventory reservation ledger shadow transition failed: order_id=%s target=%s err=%v", orderID, target, err)
			return nil
		}
		return err
	}
	placeholders := make([]string, len(allowed))
	args := make([]any, 0, len(allowed)+2)
	args = append(args, strings.ToUpper(string(target)), orderID)
	for idx, status := range allowed {
		placeholders[idx] = "?"
		args = append(args, strings.ToUpper(string(status)))
	}
	query := "UPDATE inventory_reservation SET status = ?, version = version + 1 WHERE order_id = ? AND status IN (" + strings.Join(placeholders, ",") + ")"
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		err = apperror.Wrap(apperror.CodeInternal, "transition inventory reservation ledger failed", err)
		if mode == reservationLedgerModeShadow {
			log.Printf("inventory reservation ledger shadow transition failed: order_id=%s target=%s err=%v", orderID, target, err)
			return nil
		}
		return err
	}
	return nil
}

func (r *RedisMySQLRepository) ensureReservationLedgerTable(ctx context.Context) error {
	r.reservationLedgerOnce.Do(func() {
		if r.db == nil {
			return
		}
		_, r.reservationLedgerErr = r.db.ExecContext(ctx, reservationLedgerDDL)
		if r.reservationLedgerErr != nil {
			r.reservationLedgerErr = apperror.Wrap(apperror.CodeInternal, "ensure inventory reservation ledger failed", r.reservationLedgerErr)
		}
	})
	return r.reservationLedgerErr
}

func normalizeReservationLedgerMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case reservationLedgerModeShadow:
		return reservationLedgerModeShadow
	case reservationLedgerModeEnforce:
		return reservationLedgerModeEnforce
	default:
		return reservationLedgerModeOff
	}
}

func expectedAvailable(mysqlTotal, redisReserved, ledgerReserved int64, mode string) int64 {
	reserved := expectedReserved(redisReserved, ledgerReserved, mode)
	available := mysqlTotal - reserved
	if available < 0 {
		return 0
	}
	return available
}

func expectedReserved(redisReserved, ledgerReserved int64, mode string) int64 {
	reserved := redisReserved
	switch normalizeReservationLedgerMode(mode) {
	case reservationLedgerModeShadow:
		if ledgerReserved > reserved {
			reserved = ledgerReserved
		}
	case reservationLedgerModeEnforce:
		reserved = ledgerReserved
	}
	if reserved < 0 {
		return 0
	}
	return reserved
}

func (r *RedisMySQLRepository) activeReservationTotal(ctx context.Context, productID int64) (int64, error) {
	mode := normalizeReservationLedgerMode(r.reservationLedgerMode)
	if r.db == nil || mode == reservationLedgerModeOff {
		return 0, nil
	}
	if err := r.ensureReservationLedgerTable(ctx); err != nil {
		if mode == reservationLedgerModeShadow {
			log.Printf("inventory reservation ledger shadow read failed: product_id=%d err=%v", productID, err)
			return 0, nil
		}
		return 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM inventory_reservation WHERE product_id = ? AND status = 'RESERVED'", productID).Scan(&total); err != nil {
		err = apperror.Wrap(apperror.CodeInternal, "read active inventory reservations failed", err)
		if mode == reservationLedgerModeShadow {
			log.Printf("inventory reservation ledger shadow read failed: product_id=%d err=%v", productID, err)
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}

func (r *RedisMySQLRepository) loadReservationLedger(ctx context.Context, orderID string) (reservationLedgerRecord, bool, error) {
	mode := normalizeReservationLedgerMode(r.reservationLedgerMode)
	if r.db == nil || mode == reservationLedgerModeOff || orderID == "" {
		return reservationLedgerRecord{}, false, nil
	}
	if err := r.ensureReservationLedgerTable(ctx); err != nil {
		if mode == reservationLedgerModeShadow {
			log.Printf("inventory reservation ledger shadow load failed: order_id=%s err=%v", orderID, err)
			return reservationLedgerRecord{}, false, nil
		}
		return reservationLedgerRecord{}, false, err
	}
	var record reservationLedgerRecord
	var status string
	record.OrderID = orderID
	err := r.db.QueryRowContext(ctx,
		"SELECT product_id, quantity, shard_index, status, expires_at, request_id, trace_id FROM inventory_reservation WHERE order_id = ?",
		orderID,
	).Scan(&record.ProductID, &record.Quantity, &record.ShardIndex, &status, &record.ExpiresAt, &record.RequestID, &record.TraceID)
	if err == sql.ErrNoRows {
		return reservationLedgerRecord{}, false, nil
	}
	if err != nil {
		err = apperror.Wrap(apperror.CodeInternal, "load inventory reservation ledger failed", err)
		if mode == reservationLedgerModeShadow {
			log.Printf("inventory reservation ledger shadow load failed: order_id=%s err=%v", orderID, err)
			return reservationLedgerRecord{}, false, nil
		}
		return reservationLedgerRecord{}, false, err
	}
	record.Status = domain.ReservationStatus(strings.ToLower(status))
	return record, true, nil
}

const reservationLedgerDDL = `
CREATE TABLE IF NOT EXISTS inventory_reservation (
  order_id varchar(64) NOT NULL,
  product_id bigint NOT NULL,
  quantity bigint NOT NULL,
  shard_index int NOT NULL,
  status varchar(16) NOT NULL,
  expires_at datetime(6) NOT NULL,
  version bigint NOT NULL DEFAULT 0,
  request_id varchar(64) NOT NULL DEFAULT '',
  trace_id varchar(64) NOT NULL DEFAULT '',
  create_time datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  update_time datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (order_id),
  KEY ix_status_expires (status, expires_at),
  KEY ix_product_status (product_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
`
