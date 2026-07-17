package cache

import (
	"context"
	"encoding/json"

	redis "github.com/redis/go-redis/v9"
)

type invalidationMessage struct {
	Keys     []string `json:"keys,omitempty"`
	Prefixes []string `json:"prefixes,omitempty"`
}

func (c *Coordinator) Start(ctx context.Context) {
	if !c.config.EnableL2 || c.redis == nil {
		return
	}
	c.mu.Lock()
	if c.pubsub != nil {
		c.mu.Unlock()
		return
	}
	subscribeCtx, cancel := context.WithCancel(ctx)
	pubsub := c.redis.Subscribe(subscribeCtx, c.invalidationChannel())
	receiveCtx, receiveCancel := c.operationContext(subscribeCtx)
	_, receiveErr := pubsub.Receive(receiveCtx)
	receiveCancel()
	if receiveErr != nil {
		cancel()
		_ = pubsub.Close()
		c.mu.Unlock()
		return
	}
	c.pubsub = pubsub
	c.cancel = cancel
	c.mu.Unlock()
	go c.consumeInvalidations(subscribeCtx, pubsub.Channel())
}

func (c *Coordinator) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.pubsub == nil {
		return nil
	}
	err := c.pubsub.Close()
	c.pubsub = nil
	return err
}

func (c *Coordinator) Invalidate(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	c.local.delete(keys...)
	if !c.config.EnableL2 || c.redis == nil {
		return nil
	}
	opCtx, cancel := c.operationContext(ctx)
	defer cancel()
	redisKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		redisKeys = append(redisKeys, c.redisKey(key))
	}
	if err := c.redis.Del(opCtx, redisKeys...).Err(); err != nil {
		return err
	}
	invalidationTotal.Inc()
	payload, err := json.Marshal(invalidationMessage{Keys: keys})
	if err != nil {
		return err
	}
	return c.redis.Publish(opCtx, c.invalidationChannel(), payload).Err()
}

func (c *Coordinator) InvalidatePrefix(ctx context.Context, prefixes ...string) error {
	if len(prefixes) == 0 {
		return nil
	}
	c.local.deletePrefix(prefixes...)
	if !c.config.EnableL2 || c.redis == nil {
		return nil
	}
	opCtx, cancel := c.operationContext(ctx)
	defer cancel()
	for _, prefix := range prefixes {
		var cursor uint64
		for {
			keys, next, err := c.redis.Scan(opCtx, cursor, c.redisKey(prefix)+"*", 100).Result()
			if err != nil {
				return err
			}
			if len(keys) > 0 {
				if err := c.redis.Del(opCtx, keys...).Err(); err != nil {
					return err
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	payload, err := json.Marshal(invalidationMessage{Prefixes: prefixes})
	if err != nil {
		return err
	}
	invalidationTotal.Inc()
	return c.redis.Publish(opCtx, c.invalidationChannel(), payload).Err()
}

func (c *Coordinator) consumeInvalidations(ctx context.Context, messages <-chan *redis.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			var invalidation invalidationMessage
			if json.Unmarshal([]byte(message.Payload), &invalidation) == nil {
				c.local.delete(invalidation.Keys...)
				c.local.deletePrefix(invalidation.Prefixes...)
			}
		}
	}
}

func (c *Coordinator) invalidationChannel() string {
	return c.config.Prefix + ":invalidate"
}
