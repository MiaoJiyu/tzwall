package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// RateLimiter 基于缓存的按端点/用户/IP 限流。
type RateLimiter struct {
	cache cache.Cache
	store *service.ConfigStore
}

func NewRateLimiter(c cache.Cache, store *service.ConfigStore) *RateLimiter {
	return &RateLimiter{cache: c, store: store}
}

// Limit scope 区分限流维度；cfgKey 对应系统配置键；def 为默认值（每会话窗口）。
func (r *RateLimiter) Limit(scope, cfgKey string, def int) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := r.store.Int(cfgKey, def)
		if limit <= 0 {
			c.Next()
			return
		}
		uid := "anon"
		if u := CurrentUser(c); u != nil {
			uid = strconv.Itoa(int(u.ID))
		}
		ip := ClientIP(c)
		key := "rl:" + scope + ":" + uid + ":" + ip
		n := r.cache.Incr(key, time.Hour)
		if n > int64(limit) {
			response.FailCode(c, errs.CodeTooMany, "")
			c.Abort()
			return
		}
		c.Next()
	}
}
