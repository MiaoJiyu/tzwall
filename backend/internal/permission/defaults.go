package permission

import (
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
)

// EnsureDefaults 按端点目录初始化角色默认权限集（幂等，仅补缺失，不覆盖后台已改配置）。
// 新用户自动套用对应角色的默认权限集。
func EnsureDefaults(db *gorm.DB) error {
	for _, e := range Endpoints {
		specs := []struct {
			role  string
			allow bool
		}{
			{RolePublic, e.Public},
			// 公开端点对所有角色放行，避免登录用户命中 deny 行
			{string(model.RoleUser), e.Public || hasRole(e.DefaultRoles, model.RoleUser)},
			{string(model.RoleAdmin), e.Public || hasRole(e.DefaultRoles, model.RoleAdmin)},
		}
		for _, sp := range specs {
			var cnt int64
			db.Model(&model.PermissionGrant{}).
				Where("role = ? AND user_id IS NULL AND endpoint_key = ?", sp.role, e.Key).
				Count(&cnt)
			if cnt == 0 {
				if err := db.Create(&model.PermissionGrant{
					Role:        sp.role,
					EndpointKey: e.Key,
					Allow:       sp.allow,
					Rps:         e.Rps,
					Rpm:         e.Rpm,
					RpHour:      e.RpHour,
					RpDay:       e.RpDay,
				}).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func hasRole(rs []model.Role, r model.Role) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
