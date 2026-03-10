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
)

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
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureChannelLocked(); err != nil {
		return err
	}

	if err := p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Type:         messageType,
		MessageId:    messageID,
		Body:         body,
	}); err != nil {
		p.resetLocked()
		return err
	}

	select {
	case c := <-p.confirmCh:
		if !c.Ack {
			return fmt.Errorf("rabbitmq publish nack: message_id=%s", messageID)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(confirmWaitTimeout):
		return fmt.Errorf("rabbitmq publish confirm timeout: message_id=%s", messageID)
	}
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
		p.confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
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
