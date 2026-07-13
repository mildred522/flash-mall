package job

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/order/rpc/internal/svc"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type paidProjectionEvent struct {
	EventID string `json:"event_id"`
	OrderID string `json:"order_id"`
}

type OrderPaidProjectionConsumer struct {
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOrderPaidProjectionConsumer(svcCtx *svc.ServiceContext) *OrderPaidProjectionConsumer {
	return &OrderPaidProjectionConsumer{svcCtx: svcCtx, Logger: logx.WithContext(context.Background())}
}

func (c *OrderPaidProjectionConsumer) Start() {
	if strings.TrimSpace(c.svcCtx.Config.RabbitMQURL) == "" || strings.TrimSpace(c.svcCtx.Config.RabbitMQExchange) == "" {
		return
	}
	go func() {
		for {
			if err := c.consumeOnce(); err != nil {
				c.Errorf("paid projection consumer: %v", err)
				time.Sleep(3 * time.Second)
			}
		}
	}()
}

func (c *OrderPaidProjectionConsumer) consumeOnce() error {
	conn, err := amqp.Dial(c.svcCtx.Config.RabbitMQURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err = ch.ExchangeDeclare(c.svcCtx.Config.RabbitMQExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	q, err := ch.QueueDeclare("order.events.paid.projection.q", true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err = ch.QueueBind(q.Name, "order.paid", c.svcCtx.Config.RabbitMQExchange, false, nil); err != nil {
		return err
	}
	deliveries, err := ch.Consume(q.Name, "order-paid-projection", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for d := range deliveries {
		if err := c.handle(d); err != nil {
			_ = d.Nack(false, true)
			continue
		}
		_ = d.Ack(false)
	}
	return fmt.Errorf("paid projection deliveries closed")
}

func (c *OrderPaidProjectionConsumer) handle(d amqp.Delivery) error {
	var event paidProjectionEvent
	if err := json.Unmarshal(d.Body, &event); err != nil || strings.TrimSpace(event.OrderID) == "" {
		return fmt.Errorf("invalid order.paid event: %w", err)
	}
	eventID := strings.TrimSpace(d.MessageId)
	if eventID == "" {
		sum := sha1.Sum(d.Body)
		eventID = hex.EncodeToString(sum[:])
	}
	db, err := c.svcCtx.SqlConn.RawDB()
	if err != nil {
		return err
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS order_operation_projection (order_id varchar(64) NOT NULL, paid_count bigint NOT NULL DEFAULT 0, updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY(order_id))`,
		`CREATE TABLE IF NOT EXISTS mall_product.product_search_projection (product_id bigint NOT NULL PRIMARY KEY, name varchar(255) NOT NULL DEFAULT '', searchable tinyint NOT NULL DEFAULT 1, updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS inventory_async_audit (event_id varchar(128) NOT NULL PRIMARY KEY, order_id varchar(64) NOT NULL, task varchar(64) NOT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS order_paid_projection_event (event_id varchar(128) NOT NULL PRIMARY KEY, order_id varchar(64) NOT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if _, err = db.ExecContext(context.Background(), statement); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(context.Background(), `INSERT IGNORE INTO order_paid_projection_event(event_id,order_id) VALUES (?,?)`, eventID, event.OrderID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	_, err = tx.ExecContext(context.Background(), `INSERT INTO order_operation_projection(order_id, paid_count) VALUES (?,1) ON DUPLICATE KEY UPDATE paid_count=paid_count+1`, event.OrderID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(context.Background(), `INSERT IGNORE INTO inventory_async_audit(event_id,order_id,task) VALUES (?,?,'stock_audit')`, eventID, event.OrderID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(context.Background(), `INSERT INTO mall_product.product_search_projection(product_id,name,searchable) SELECT p.id,p.name,p.status FROM mall_order.orders o JOIN mall_product.product p ON p.id=o.product_id WHERE o.id=? ON DUPLICATE KEY UPDATE name=VALUES(name),searchable=VALUES(searchable)`, event.OrderID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(context.Background(), `UPDATE mall_product.product_card_snapshot s JOIN mall_order.orders o ON o.product_id=s.product_id SET s.stock_available=(SELECT COALESCE(SUM(stock),0) FROM mall_product.product_stock_bucket WHERE product_id=o.product_id), s.version=s.version+1 WHERE o.id=?`, event.OrderID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
