package service

import (
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/permission"
)

// GrantInput 权限矩阵单条输入。
type GrantInput struct {
	EndpointKey string `json:"endpoint_key"`
	Allow       bool   `json:"allow"`
	Rps         int    `json:"rps"`
	Rpm         int    `json:"rpm"`
	RpHour      int    `json:"rphour"`
	RpDay       int    `json:"rpday"`
}

// PermissionService 权限矩阵读写（仅超级管理员调用）。
type PermissionService struct {
	db  *gorm.DB
	res *permission.Resolver
}

func NewPermission(db *gorm.DB, res *permission.Resolver) *PermissionService {
	return &PermissionService{db: db, res: res}
}

// Matrix 返回某角色（user_id=0）或某用户覆盖的完整矩阵（含未配置项，默认拒绝）。
func (s *PermissionService) Matrix(role string, userID uint) ([]model.PermissionGrant, error) {
	existing := map[string]model.PermissionGrant{}
	var rows []model.PermissionGrant
	q := s.db.Where("endpoint_key IN ?", permission.Keys())
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	} else {
		q = q.Where("role = ? AND user_id IS NULL", role)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		existing[r.EndpointKey] = r
	}

	out := make([]model.PermissionGrant, 0, len(permission.Endpoints))
	for _, key := range permission.Keys() {
		if g, ok := existing[key]; ok {
			g.UserID = nilOrID(userID)
			out = append(out, g)
			continue
		}
		out = append(out, model.PermissionGrant{
			Role:        role,
			UserID:      nilOrID(userID),
			EndpointKey: key,
			Allow:       false,
		})
	}
	return out, nil
}

func nilOrID(id uint) *uint {
	if id == 0 {
		return nil
	}
	v := id
	return &v
}

// Save 批量保存矩阵：按 (role,user_id,endpoint_key) 更新或新增，写审计并失效缓存。
func (s *PermissionService) Save(actor *model.User, role string, userID uint, items []GrantInput) error {
	for _, it := range items {
		if _, ok := permission.Get(it.EndpointKey); !ok {
			continue // 忽略目录外端点
		}
		var g model.PermissionGrant
		q := s.db.Where("endpoint_key = ?", it.EndpointKey)
		if userID > 0 {
			q = q.Where("user_id = ?", userID)
		} else {
			q = q.Where("role = ? AND user_id IS NULL", role)
		}
		err := q.First(&g).Error
		if err != nil {
			g = model.PermissionGrant{Role: role, UserID: nilOrID(userID), EndpointKey: it.EndpointKey}
		}
		g.Allow = it.Allow
		g.Rps = it.Rps
		g.Rpm = it.Rpm
		g.RpHour = it.RpHour
		g.RpDay = it.RpDay
		if g.ID == 0 {
			if err := s.db.Create(&g).Error; err != nil {
				return err
			}
		} else {
			if err := s.db.Save(&g).Error; err != nil {
				return err
			}
		}
	}
	if s.res != nil {
		s.res.Invalidate()
	}
	target := role
	if userID > 0 {
		target = "user:" + uintToStr(userID)
	}
	var aid uint
	if actor != nil {
		aid = actor.ID
	}
	Audit(s.db, aid, "permission.update", "permission", target, "更新权限矩阵")
	return nil
}
