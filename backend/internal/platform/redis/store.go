package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store wraps a redis.Client with the simple key/value operations used by
// application services (e.g. refresh-token storage).
type Store struct {
	client *redis.Client
}

// NewStore wraps the given client in a Store.
func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

// Set stores value at key with the given TTL.
func (s *Store) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

// Get retrieves the value stored at key.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("redis get %q: %w", key, err)
	}
	return val, nil
}

// Del removes the key.
func (s *Store) Del(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del %q: %w", key, err)
	}
	return nil
}
