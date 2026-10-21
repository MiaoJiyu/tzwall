package service

import (
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
)

// Audit 写入审计日志（配置修改、处罚、审核、权限变更等）。
func Audit(db *gorm.DB, actorID uint, action, objType, objID, detail string) {
	_ = db.Create(&model.AuditLog{
		ActorID:    actorID,
		Action:     action,
		ObjectType: objType,
		ObjectID:   objID,
		Detail:     detail,
	}).Error
}
