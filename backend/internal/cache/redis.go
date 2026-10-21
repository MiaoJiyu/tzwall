package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"tzwall/backend/internal/config"
)

type redisCache struct {
	client *redis.Client
}

func newRedis(cfg *config.Config) (*redisCache, error) {
	c := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &redisCache{client: c}, nil
}

func (r *redisCache) Get(key string) (string, bool) {
	v, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (r *redisCache) Set(key string, val string, ttl time.Duration) {
	r.client.Set(context.Background(), key, val, ttl)
}

func (r *redisCache) Incr(key string, ttl time.Duration) int64 {
	ctx := context.Background()
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0
	}
	if n == 1 && ttl > 0 {
		r.client.Expire(ctx, key, ttl)
	}
	return n
}

func (r *redisCache) Del(key string) {
	r.client.Del(context.Background(), key)
}

func (r *redisCache) SAdd(key string, member string, ttl time.Duration) {
	ctx := context.Background()
	r.client.SAdd(ctx, key, member)
	if ttl > 0 {
		if n, _ := r.client.TTL(ctx, key).Result(); n < 0 {
			r.client.Expire(ctx, key, ttl)
		}
	}
}

func (r *redisCache) SIsMember(key string, member string) bool {
	ok, err := r.client.SIsMember(context.Background(), key, member).Result()
	return err == nil && ok
}

func (r *redisCache) Close() error {
	return r.client.Close()
}
