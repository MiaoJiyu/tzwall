package cache

import (
	"log/slog"
	"time"

	"tzwall/backend/internal/config"
)

// Cache 统一缓存/计数接口。本地无 Redis 时自动降级为内存实现。
type Cache interface {
	Get(key string) (string, bool)
	Set(key string, val string, ttl time.Duration)
	Incr(key string, ttl time.Duration) int64
	Del(key string)
	SAdd(key string, member string, ttl time.Duration)
	SIsMember(key string, member string) bool
	Close() error
}

// New 按能力选择实现；Redis 配置但不可用则自动降级内存。
func New(cfg *config.Config) (Cache, error) {
	if cfg.RedisConfigured {
		if r, err := newRedis(cfg); err == nil {
			return r, nil
		} else {
			slog.Default().Warn("Redis 不可用，降级为内存缓存", "err", err.Error())
		}
	}
	return newMemory(), nil
}
