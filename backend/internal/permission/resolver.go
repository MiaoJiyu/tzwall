package permission

import (
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
)

// Effective 端点生效权限：是否允许与四档速率（0 表示不限）。
type Effective struct {
	Allow  bool
	Rps    int
	Rpm    int
	RpHour int
	RpDay  int
}

// Resolver 权限解析：用户覆盖 > 角色默认权限集 > 公开白名单 > 默认拒绝。
// 超级管理员直通且不受速率限制。解析结果内存缓存 60 秒，后台写入即失效。
type Resolver struct {
	db    *gorm.DB
	mu    sync.RWMutex
	cache map[string]entry
}

type entry struct {
	eff Effective
	at  time.Time
}

const cacheTTL = 60 * time.Second

func NewResolver(db *gorm.DB) *Resolver {
	return &Resolver{db: db, cache: map[string]entry{}}
}

// RoleKey 返回用户的角色键；未登录为 public。
func RoleKey(u *model.User) string {
	if u == nil {
		return RolePublic
	}
	return string(u.Role)
}

// Resolve 解析某用户对某端点的生效权限。
func (r *Resolver) Resolve(u *model.User, key string) Effective {
	if u != nil && u.IsSuperAdmin() {
		return Effective{Allow: true}
	}
	role := RoleKey(u)
	uid := uint(0)
	if u != nil {
		uid = u.ID
	}
	ck := fmt.Sprintf("%s|u%d|r%s", key, uid, role)

	r.mu.RLock()
	if e, ok := r.cache[ck]; ok && time.Since(e.at) < cacheTTL {
		r.mu.RUnlock()
		return e.eff
	}
	r.mu.RUnlock()

	eff := r.query(uid, role, key)
	r.mu.Lock()
	r.cache[ck] = entry{eff: eff, at: time.Now()}
	r.mu.Unlock()
	return eff
}

func (r *Resolver) query(uid uint, role, key string) Effective {
	var g model.PermissionGrant
	// 1) 用户单独覆盖
	if uid > 0 {
		if err := r.db.Where("user_id = ? AND endpoint_key = ?", uid, key).First(&g).Error; err == nil {
			return fromGrant(g)
		}
	}
	// 2) 公开白名单：无论是否登录都放行（仅取 public 行的速率配置）。
	// 必须先于角色默认权限判定，否则登录用户会命中 ensureDefaults 写入的
	// role=user/admin 的 deny 行而被错误拒绝。
	if e, ok := Get(key); ok && e.Public {
		var pg model.PermissionGrant
		if err := r.db.Where("role = ? AND user_id IS NULL AND endpoint_key = ?", RolePublic, key).First(&pg).Error; err == nil {
			eff := fromGrant(pg)
			eff.Allow = true // 公开端点始终放行，仅取速率配置
			return eff
		}
		return Effective{Allow: true}
	}
	// 3) 角色默认权限集
	if err := r.db.Where("role = ? AND user_id IS NULL AND endpoint_key = ?", role, key).First(&g).Error; err == nil {
		return fromGrant(g)
	}
	// 4) 默认拒绝
	return Effective{}
}

func fromGrant(g model.PermissionGrant) Effective {
	return Effective{Allow: g.Allow, Rps: g.Rps, Rpm: g.Rpm, RpHour: g.RpHour, RpDay: g.RpDay}
}

// Invalidate 清空解析缓存（后台保存矩阵后调用）。
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	r.cache = map[string]entry{}
	r.mu.Unlock()
}

// AllowedPages 计算当前用户可访问的页面键集合。
func (r *Resolver) AllowedPages(u *model.User) []string {
	out := make([]string, 0, len(Pages))
	for _, p := range Pages {
		if p.Public {
			out = append(out, p.Key)
			continue
		}
		if u == nil {
			continue
		}
		ok := true
		for _, req := range p.Requires {
			if !r.Resolve(u, req).Allow {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, p.Key)
		}
	}
	return out
}
