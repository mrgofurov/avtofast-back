package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	goredis "github.com/redis/go-redis/v9"
)

type Store struct {
	client *goredis.Client
}

func New(client *goredis.Client) *Store {
	return &Store{client: client}
}

// Ensure interfaces are satisfied
var _ domain.IdempotencyStore = (*Store)(nil)
var _ domain.RateLimiterStore = (*Store)(nil)
var _ domain.CacheStore = (*Store)(nil)

// Idempotency
func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := s.client.Get(ctx, "idemp:"+key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}

func (s *Store) Set(ctx context.Context, key string, responseData []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return s.client.Set(ctx, "idemp:"+key, responseData, ttl).Err()
}

// Rate Limiting (sliding counter)
func (s *Store) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	rKey := "ratelimit:" + key
	pipe := s.client.TxPipeline()
	incr := pipe.Incr(ctx, rKey)
	pipe.Expire(ctx, rKey, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, limit, 0, nil // fail open on redis error
	}

	count := int(incr.Val())
	if count > limit {
		ttl, _ := s.client.TTL(ctx, rKey).Result()
		return false, 0, ttl, nil
	}

	return true, limit - count, 0, nil
}

// General Cache
func (s *Store) CacheGet(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, "cache:"+key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, fmt.Errorf("cache miss: %s", key)
		}
		return nil, err
	}
	return val, nil
}

func (s *Store) CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return s.client.Set(ctx, "cache:"+key, val, ttl).Err()
}

func (s *Store) CacheDelete(ctx context.Context, key string) error {
	return s.client.Del(ctx, "cache:"+key).Err()
}

func (s *Store) Delete(ctx context.Context, key string) error {
	return s.CacheDelete(ctx, key)
}
