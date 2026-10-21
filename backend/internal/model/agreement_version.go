package model

import "time"

// AgreementVersion 协议历史版本：发布时落库，支持查看与回滚。
type AgreementVersion struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"size:32;index:idx_agreement_versions_key_created,priority:1" json:"key"`
	Version     string    `gorm:"size:32" json:"version"`
	Title       string    `gorm:"size:128" json:"title"`
	Content     string    `gorm:"type:text" json:"content"`
	EffectiveAt time.Time `json:"effective_at"`
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `gorm:"index:idx_agreement_versions_key_created,priority:2" json:"created_at"`
}
