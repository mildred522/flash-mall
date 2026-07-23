package ordermysql

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/application/adminops"
)

func (r *AdminOpsRepository) Events(ctx context.Context, query adminops.EventQuery) (adminops.EventList, error) {
	where := "1=1"
	args := make([]any, 0, 3)
	if query.Status >= 0 {
		where += " AND status = ?"
		args = append(args, query.Status)
	}
	if query.EventType != "" {
		where += " AND event_type = ?"
		args = append(args, query.EventType)
	}
	if query.AggregateID != "" {
		where += " AND aggregate_id = ?"
		args = append(args, query.AggregateID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM order_outbox WHERE "+where, args...).Scan(&total); err != nil {
		return adminops.EventList{}, err
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT id, event_id, event_type, aggregate_id, status, attempt_count, last_error,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(update_time, '%Y-%m-%d %H:%i:%s'), '')
FROM order_outbox WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return adminops.EventList{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]adminops.Event, 0)
	for rows.Next() {
		var item adminops.Event
		if err := rows.Scan(&item.ID, &item.EventID, &item.EventType, &item.AggregateID, &item.Status,
			&item.AttemptCount, &item.LastError, &item.CreateTime, &item.UpdateTime); err != nil {
			return adminops.EventList{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return adminops.EventList{}, err
	}
	return adminops.EventList{Items: items, Total: total}, nil
}

func (r *AdminOpsRepository) RetryEvent(ctx context.Context, eventID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE order_outbox SET status = 0, next_retry_at = NOW(), last_error = '' WHERE event_id = ?", eventID)
	return err
}
