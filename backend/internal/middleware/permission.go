package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/permission"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
)

// PermissionMW 端点权限与速率中间件。
// 未授权端点返回 41011（前端提示“请求端点失败”），超限返回 429。
type PermissionMW struct {
	res   *permission.Resolver
	cache cache.Cache
}

func NewPermission(res *permission.Resolver, c cache.Cache) *PermissionMW {
	return &PermissionMW{res: res, cache: c}
}

// Public 公开端点（未登录也可按权限解析放行）。
func (p *PermissionMW) Public(key string) gin.HandlerFunc { return p.check(key, false) }

// Require 登录后端点。
func (p *PermissionMW) Require(key string) gin.HandlerFunc { return p.check(key, true) }

func (p *PermissionMW) check(key string, needLogin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if needLogin && u == nil {
			response.FailCode(c, errs.CodeUnauthorized, "")
			c.Abort()
			return
		}
		eff := p.res.Resolve(u, key)
		if !eff.Allow {
			response.FailEndpoint(c, key)
			c.Abort()
			return
		}
		if !p.rateOK(c, u, key, eff) {
			response.FailCode(c, errs.CodeTooMany, "")
			c.Abort()
			return
		}
		c.Set("endpoint_key", key)
		c.Next()
	}
}

func (p *PermissionMW) rateOK(c *gin.Context, u *model.User, key string, eff permission.Effective) bool {
	if p.cache == nil {
		return true
	}
	uid := "anon"
	if u != nil {
		uid = strconv.Itoa(int(u.ID))
	}
	ip := ClientIP(c)
	windows := []struct {
		limit int
		dur   time.Duration
		name  string
	}{
		{eff.Rps, time.Second, "s"},
		{eff.Rpm, time.Minute, "m"},
		{eff.RpHour, time.Hour, "h"},
		{eff.RpDay, 24 * time.Hour, "d"},
	}
	for _, w := range windows {
		if w.limit <= 0 {
			continue
		}
		ck := fmt.Sprintf("rl:ep:%s:%s:%s:%s", key, uid, ip, w.name)
		if p.cache.Incr(ck, w.dur) > int64(w.limit) {
			return false
		}
	}
	return true
}
