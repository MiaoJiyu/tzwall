package model

import (
	"time"
)

// Agreement 协议文本（用户/隐私/数据保护），带版本与生效时间。
type Agreement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"uniqueIndex;size:32" json:"key"` // user/privacy/data
	Title       string    `gorm:"size:128" json:"title"`
	Version     string    `gorm:"size:32" json:"version"`
	Content     string    `gorm:"type:text" json:"content"`
	EffectiveAt time.Time `json:"effective_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// WordFilter 违禁/常用词表，包含匹配、不区分大小写。
type WordFilter struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Word      string    `gorm:"uniqueIndex;size:128" json:"word"`
	Category  string    `gorm:"size:32" json:"category"`
	CreatedAt time.Time `json:"created_at"`
}

// SystemConfig 系统配置键值（支持热更新）。
type SystemConfig struct {
	Key       string    `gorm:"primaryKey;size:80" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
