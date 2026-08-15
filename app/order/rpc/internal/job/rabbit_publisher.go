package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitExchangeType = "topic"
	confirmWaitTimeout = 5 * time.Second
	confirmBufferSize  = 1024
)

type RabbitMessage struct {
	RoutingKey  string
	MessageID   string
	MessageType string
	Body        []byte
}

// RabbitPublisher provides a small resilient publisher with confirm ack.
type RabbitPublisher struct {
	url      string
	exchange string

	mu             sync.Mutex
	conn           *amqp.Connection
	ch             *amqp.Channel
	confirmCh      <-chan amqp.Confirmation
	confirmEnabled bool
}

func NewRabbitPublisher(url, exchange string) *RabbitPublisher {
	return &RabbitPublisher{url: url, exchange: exchange}
}

func (p *RabbitPublisher) Publish(ctx context.Context, routingKey, messageID, messageType string, body []byte) error {
	return p.PublishBatch(ctx, []RabbitMessage{{
		RoutingKey: routingKey, MessageID: messageID, MessageType: messageType, Body: body,
	}})
}

func (p *RabbitPublisher) PublishBatch(ctx context.Context, messages []RabbitMessage) error {
	if len(messages) == 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureChannelLocked(); err != nil {
		return err
	}

	for _, message := range messages {
		if err := p.ch.PublishWithContext(ctx, p.exchange, message.RoutingKey, false, false, amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			ContentType:  "application/json",
			Type:         message.MessageType,
			MessageId:    message.MessageID,
			Body:         message.Body,
		}); err != nil {
			p.resetLocked()
			return err
		}
	}

	timer := time.NewTimer(confirmWaitTimeout)
	defer timer.Stop()
	for confirmed := 0; confirmed < len(messages); confirmed++ {
		select {
		case confirmation, ok := <-p.confirmCh:
			if !ok {
				p.resetLocked()
				return fmt.Errorf("rabbitmq publish confirm channel closed")
			}
			if !confirmation.Ack {
				p.resetLocked()
				return fmt.Errorf("rabbitmq publish nack: delivery_tag=%d", confirmation.DeliveryTag)
			}
		case <-ctx.Done():
			p.resetLocked()
			return ctx.Err()
		case <-timer.C:
			p.resetLocked()
			return fmt.Errorf("rabbitmq publish confirm timeout: batch_size=%d confirmed=%d", len(messages), confirmed)
		}
	}
	return nil
}

func (p *RabbitPublisher) ensureChannelLocked() error {
	if p.ch != nil {
		return nil
	}

	conn, err := amqp.Dial(p.url)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	if err := ch.ExchangeDeclare(
		p.exchange,
		rabbitExchangeType,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	if !p.confirmEnabled {
		if err := ch.Confirm(false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
		p.confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, confirmBufferSize))
	}

	p.conn = conn
	p.ch = ch
	p.confirmEnabled = true
	return nil
}

func (p *RabbitPublisher) resetLocked() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.ch = nil
	p.conn = nil
	p.confirmCh = nil
	p.confirmEnabled = false
}

func (p *RabbitPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resetLocked()
}
