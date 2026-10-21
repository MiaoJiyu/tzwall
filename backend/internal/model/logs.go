package model

import (
	"time"
)

// AccessLog 访问日志：接近全量记录，敏感字段脱敏后落库，IP 仅超级管理员可见。
type AccessLog struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	Method      string    `gorm:"size:16" json:"-"`
	Path        string    `gorm:"size:255" json:"-"`
	Query       string    `gorm:"type:text" json:"-"`
	Headers     string    `gorm:"type:text" json:"-"` // 关键请求头（已脱敏）
	BodyPreview string    `gorm:"type:text" json:"-"` // 请求体摘要（已脱敏）
	BodyDigest  string    `gorm:"size:64" json:"-"`   // 请求体 SHA-256
	UA          string    `gorm:"size:512" json:"-"`
	IP          string    `gorm:"size:64;index" json:"-"`
	UserID      uint      `gorm:"index" json:"-"`
	StatusCode  int       `json:"-"`
	DurationMs  int64     `json:"-"`
	CreatedAt   time.Time `gorm:"index" json:"-"`
}

// AuditLog 审计日志：配置修改、处罚、审核、权限变更等。
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"-"`
	ActorID    uint      `gorm:"index" json:"-"`
	Action     string    `gorm:"size:64" json:"-"`
	ObjectType string    `gorm:"size:32" json:"-"`
	ObjectID   string    `gorm:"size:64" json:"-"`
	Detail     string    `gorm:"type:text" json:"-"`
	CreatedAt  time.Time `gorm:"index" json:"-"`
}
