package dedupe

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDeduplicator struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisDeduplicator(addr string, ttl time.Duration) (*RedisDeduplicator, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &RedisDeduplicator{
		client: client,
		ttl:    ttl,
	}, nil
}

func (r *RedisDeduplicator) IsDuplicate(ctx context.Context, eventID string) (bool, error) {
	key := fmt.Sprintf("event:%s", eventID)

	// SETNX returns true if key was set (not duplicate), false if key exists (duplicate)
	wasSet, err := r.client.SetNX(ctx, key, "1", r.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx: %w", err)
	}

	// If wasSet is false, the key already exists (duplicate)
	return !wasSet, nil
}

func (r *RedisDeduplicator) Close() error {
	return r.client.Close()
}
