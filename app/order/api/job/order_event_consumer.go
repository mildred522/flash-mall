package job

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/svc"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type OrderEventConsumer struct {
	svcCtx *svc.ServiceContext
	logx.Logger
}

type orderCreatedEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	OrderID   string `json:"order_id"`
	RequestID string `json:"request_id"`
	UserID    int64  `json:"user_id"`
	ProductID int64  `json:"product_id"`
	Amount    int64  `json:"amount"`
	CreatedAt int64  `json:"created_at"`
}

func NewOrderEventConsumer(svcCtx *svc.ServiceContext) *OrderEventConsumer {
	return &OrderEventConsumer{
		svcCtx: svcCtx,
		Logger: logx.WithContext(context.Background()),
	}
}

func (c *OrderEventConsumer) Start() {
	if !c.svcCtx.Config.OrderEventConsumerEnabled {
		c.Info("order event consumer disabled by config")
		return
	}
	if strings.TrimSpace(c.svcCtx.Config.RabbitMQURL) == "" {
		c.Info("order event consumer disabled: rabbitmq url empty")
		return
	}
	if strings.TrimSpace(c.svcCtx.Config.RabbitMQExchange) == "" {
		c.Info("order event consumer disabled: rabbitmq exchange empty")
		return
	}

	c.Infof("order event consumer started, queue=%s", c.queueName())
	go func() {
		for {
			if err := c.consumeOnce(); err != nil {
				c.Errorf("order event consume failed: %v", err)
				time.Sleep(3 * time.Second)
			}
		}
	}()
}

func (c *OrderEventConsumer) consumeOnce() error {
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

	queue, err := ch.QueueDeclare(c.queueName(), true, false, false, false, nil)
	if err != nil {
		return err
	}

	if err = ch.QueueBind(queue.Name, c.routeKey(), c.svcCtx.Config.RabbitMQExchange, false, nil); err != nil {
		return err
	}

	prefetch := c.svcCtx.Config.RabbitMQPrefetch
	if prefetch <= 0 {
		prefetch = 20
	}
	if err = ch.Qos(prefetch, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(
		queue.Name,
		c.consumerTag(),
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for d := range deliveries {
		if handleErr := c.handleDelivery(d); handleErr != nil {
			metrics.OrderEventConsumeTotal.WithLabelValues("retry").Inc()
			c.Errorf("order event handle failed: msg_id=%s err=%v", d.MessageId, handleErr)
			_ = d.Nack(false, true)
			continue
		}
		_ = d.Ack(false)
	}

	return fmt.Errorf("order event deliveries closed")
}

func (c *OrderEventConsumer) handleDelivery(d amqp.Delivery) error {
	msgID := strings.TrimSpace(d.MessageId)
	if msgID == "" {
		sum := sha1.Sum(d.Body)
		msgID = hex.EncodeToString(sum[:])
	}

	ttl := c.svcCtx.Config.EventDedupTTLSeconds
	if ttl <= 0 {
		ttl = 24 * 60 * 60
	}
	isNew, err := c.svcCtx.Redis.SetnxExCtx(context.Background(), "order:event:processed:"+msgID, "1", int(ttl))
	if err != nil {
		return err
	}
	if !isNew {
		metrics.OrderEventConsumeTotal.WithLabelValues("duplicate").Inc()
		return nil
	}

	var event orderCreatedEvent
	if err = json.Unmarshal(d.Body, &event); err != nil {
		metrics.OrderEventConsumeTotal.WithLabelValues("invalid").Inc()
		c.Errorf("order event json invalid: msg_id=%s err=%v", msgID, err)
		return nil
	}

	metrics.OrderEventConsumeTotal.WithLabelValues("success").Inc()
	c.Infow("order event consumed",
		logx.Field("event_id", event.EventID),
		logx.Field("event_type", event.EventType),
		logx.Field("order_id", event.OrderID),
		logx.Field("user_id", event.UserID),
	)
	return nil
}

func (c *OrderEventConsumer) queueName() string {
	queue := strings.TrimSpace(c.svcCtx.Config.RabbitMQQueue)
	if queue == "" {
		return "order.events.created.q"
	}
	return queue
}

func (c *OrderEventConsumer) routeKey() string {
	key := strings.TrimSpace(c.svcCtx.Config.RabbitMQRouteKey)
	if key == "" {
		return "order.created"
	}
	return key
}

func (c *OrderEventConsumer) consumerTag() string {
	tag := strings.TrimSpace(c.svcCtx.Config.RabbitMQConsumerTag)
	if tag == "" {
		return "order-api-order-event-consumer"
	}
	return tag
}
