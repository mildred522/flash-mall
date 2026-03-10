package job

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"flash-mall/app/order/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	outboxStatusPending    = 0
	outboxStatusPublished  = 1
	outboxStatusPublishing = 2
	outboxStatusDead       = 3
)

const renewLeaderLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
end
return 0
`

const releaseLeaderLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`

type outboxEvent struct {
	ID           int64
	EventID      string
	EventType    string
	Payload      string
	AttemptCount int
}

type OutboxPublisher struct {
	svcCtx *svc.ServiceContext
	logx.Logger

	rabbit *RabbitPublisher

	leaderID    string
	leaderOwned bool
}

func NewOutboxPublisher(svcCtx *svc.ServiceContext) *OutboxPublisher {
	host, _ := os.Hostname()
	if strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	return &OutboxPublisher{
		svcCtx:   svcCtx,
		Logger:   logx.WithContext(context.Background()),
		rabbit:   NewRabbitPublisher(svcCtx.Config.RabbitMQURL, svcCtx.Config.RabbitMQExchange),
		leaderID: fmt.Sprintf("%s-%d", host, time.Now().UnixNano()),
	}
}

func (p *OutboxPublisher) Start() {
	if strings.TrimSpace(p.svcCtx.Config.RabbitMQURL) == "" || strings.TrimSpace(p.svcCtx.Config.RabbitMQExchange) == "" {
		p.Info("outbox publisher disabled: rabbitmq config empty")
		return
	}

	poll := p.svcCtx.Config.OutboxPollMs
	if poll <= 0 {
		poll = 1000
	}

	p.Infof("outbox publisher started, poll_ms=%d", poll)
	go func() {
		ticker := time.NewTicker(time.Duration(poll) * time.Millisecond)
		defer ticker.Stop()
		for {
			// Use a longer timeout than poll interval to avoid batch publish timing out under load.
			ctx, cancel := context.WithTimeout(context.Background(), p.processTimeout(poll))
			leaderReady, leaderErr := p.ensureLeadership(ctx)
			if leaderErr != nil {
				p.Errorf("outbox leader check error: %v", leaderErr)
				cancel()
				<-ticker.C
				continue
			}
			if !leaderReady {
				cancel()
				<-ticker.C
				continue
			}
			if err := p.processOnce(ctx); err != nil {
				p.Errorf("outbox process error: %v", err)
			}
			cancel()
			<-ticker.C
		}
	}()
}

func (p *OutboxPublisher) processTimeout(pollMs int) time.Duration {
	batch := p.svcCtx.Config.OutboxBatchSize
	if batch <= 0 {
		batch = 20
	}

	timeoutMs := pollMs * batch
	if timeoutMs < 5000 {
		timeoutMs = 5000
	}
	if timeoutMs > 60000 {
		timeoutMs = 60000
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func (p *OutboxPublisher) ensureLeadership(ctx context.Context) (bool, error) {
	if !p.svcCtx.Config.OutboxSingleActive {
		return true, nil
	}

	lockKey := p.leaderLockKey()
	ttl := p.leaderLockTTL()
	if !p.leaderOwned {
		claimed, err := p.svcCtx.Redis.SetnxExCtx(ctx, lockKey, p.leaderID, ttl)
		if err != nil {
			return false, err
		}
		if !claimed {
			return false, nil
		}
		p.leaderOwned = true
		p.Infof("outbox publisher became leader, lock_key=%s lock_ttl=%ds", lockKey, ttl)
		return true, nil
	}

	renewed, err := p.svcCtx.Redis.EvalCtx(ctx, renewLeaderLockScript, []string{lockKey}, p.leaderID, ttl)
	if err != nil {
		return false, err
	}
	if parseRedisInt(renewed) != 1 {
		p.leaderOwned = false
		p.Infof("outbox publisher lost leadership, lock_key=%s", lockKey)
		return false, nil
	}
	return true, nil
}

func (p *OutboxPublisher) leaderLockKey() string {
	key := strings.TrimSpace(p.svcCtx.Config.OutboxLeaderLockKey)
	if key == "" {
		return "order:outbox:publisher:leader"
	}
	return key
}

func (p *OutboxPublisher) leaderLockTTL() int {
	ttl := p.svcCtx.Config.OutboxLeaderLockTTL
	if ttl < 30 {
		return 90
	}
	return ttl
}

func parseRedisInt(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case uint64:
		return int64(val)
	case string:
		if val == "1" {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (p *OutboxPublisher) processOnce(ctx context.Context) error {
	if err := p.recoverTimeoutPublishing(ctx); err != nil {
		return err
	}

	events, err := p.claimBatch(ctx)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	for _, evt := range events {
		if err := p.publishOne(ctx, evt); err != nil {
			p.Errorf("outbox publish failed: id=%d event_id=%s err=%v", evt.ID, evt.EventID, err)
			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if markErr := p.markRetry(writeCtx, evt, err); markErr != nil {
				p.Errorf("outbox mark retry failed: id=%d err=%v", evt.ID, markErr)
			}
			cancel()
			continue
		}
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := p.markPublished(writeCtx, evt.ID); err != nil {
			p.Errorf("outbox mark published failed: id=%d err=%v", evt.ID, err)
		}
		cancel()
	}

	return nil
}

func (p *OutboxPublisher) claimBatch(ctx context.Context) ([]outboxEvent, error) {
	batch := p.svcCtx.Config.OutboxBatchSize
	if batch <= 0 {
		batch = 20
	}

	db, err := p.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
SELECT id, event_id, event_type, payload, attempt_count
FROM order_outbox
WHERE status = ? AND next_retry_at <= NOW()
ORDER BY id
LIMIT ?
`, outboxStatusPending, batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimed := make([]outboxEvent, 0, batch)
	for rows.Next() {
		var evt outboxEvent
		if scanErr := rows.Scan(&evt.ID, &evt.EventID, &evt.EventType, &evt.Payload, &evt.AttemptCount); scanErr != nil {
			return nil, scanErr
		}

		res, execErr := db.ExecContext(ctx, `
UPDATE order_outbox
SET status = ?, update_time = NOW()
WHERE id = ? AND status = ?
`, outboxStatusPublishing, evt.ID, outboxStatusPending)
		if execErr != nil {
			return nil, execErr
		}
		affected, _ := res.RowsAffected()
		if affected == 1 {
			claimed = append(claimed, evt)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return claimed, nil
}

func (p *OutboxPublisher) publishOne(ctx context.Context, evt outboxEvent) error {
	routeKey := strings.TrimSpace(p.svcCtx.Config.RabbitMQRouteKey)
	if routeKey == "" {
		routeKey = "order.created"
	}
	return p.rabbit.Publish(ctx, routeKey, evt.EventID, evt.EventType, []byte(evt.Payload))
}

func (p *OutboxPublisher) markPublished(ctx context.Context, id int64) error {
	_, err := p.svcCtx.SqlConn.ExecCtx(ctx, `
UPDATE order_outbox
SET status = ?, published_at = NOW(), last_error = '', update_time = NOW()
WHERE id = ? AND status = ?
`, outboxStatusPublished, id, outboxStatusPublishing)
	return err
}

func (p *OutboxPublisher) markRetry(ctx context.Context, evt outboxEvent, publishErr error) error {
	retrySec := p.svcCtx.Config.OutboxRetrySec
	if retrySec <= 0 {
		retrySec = 5
	}
	maxRetries := p.svcCtx.Config.OutboxMaxRetries
	if maxRetries <= 0 {
		maxRetries = 8
	}

	nextAttempt := evt.AttemptCount + 1
	nextStatus := outboxStatusPending
	if nextAttempt > maxRetries {
		nextStatus = outboxStatusDead
	}

	errMsg := publishErr.Error()
	if len(errMsg) > 255 {
		errMsg = errMsg[:255]
	}

	_, err := p.svcCtx.SqlConn.ExecCtx(ctx, `
UPDATE order_outbox
SET status = ?,
    attempt_count = ?,
    next_retry_at = DATE_ADD(NOW(), INTERVAL ? SECOND),
    last_error = ?,
    update_time = NOW()
WHERE id = ? AND status = ?
`, nextStatus, nextAttempt, retrySec, errMsg, evt.ID, outboxStatusPublishing)
	if err != nil {
		return err
	}

	if nextStatus == outboxStatusDead {
		p.Errorf("outbox moved to dead status: id=%d event_id=%s", evt.ID, evt.EventID)
	}
	return nil
}

func (p *OutboxPublisher) recoverTimeoutPublishing(ctx context.Context) error {
	timeoutSec := p.svcCtx.Config.OutboxTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	_, err := p.svcCtx.SqlConn.ExecCtx(ctx, `
UPDATE order_outbox
SET status = ?,
    next_retry_at = NOW(),
    last_error = 'recovered from timeout',
    update_time = NOW()
WHERE status = ?
  AND update_time < DATE_SUB(NOW(), INTERVAL ? SECOND)
`, outboxStatusPending, outboxStatusPublishing, timeoutSec)
	return err
}

func (p *OutboxPublisher) Stop() {
	if p.svcCtx.Config.OutboxSingleActive {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = p.svcCtx.Redis.EvalCtx(ctx, releaseLeaderLockScript, []string{p.leaderLockKey()}, p.leaderID)
		cancel()
	}
	p.rabbit.Close()
}

func InsertOrderCreatedOutbox(tx *sql.Tx, orderID, payload string) error {
	eventID := fmt.Sprintf("order.created:%s", orderID)
	_, err := tx.Exec(
		`INSERT IGNORE INTO order_outbox (event_id, event_type, aggregate_id, payload, status, next_retry_at)
VALUES (?, 'order.created', ?, ?, 0, NOW())`,
		eventID, orderID, payload,
	)
	return err
}
