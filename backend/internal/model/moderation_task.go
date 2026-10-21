package model

import (
	"time"
)

// 审核任务状态
const (
	ModerationNone    = "none"    // 未审核
	ModerationPending = "pending" // 待回调或轮询
	ModerationPass    = "pass"    // 通过
	ModerationBlock   = "block"   // 违规
	ModerationReview  = "review"  // 可疑，转人工
	ModerationFailed  = "failed"  // 调用失败，转人工
)

// ModerationTask 内容审核任务：记录 bizID、对象、JobID 与原始判定，便于回调/轮询追溯。
type ModerationTask struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	BizID      string    `gorm:"size:64;index" json:"biz_id"`
	Kind       string    `gorm:"size:16" json:"kind"` // text/image
	ObjectKey  string    `gorm:"size:255" json:"object_key,omitempty"`
	TargetType string    `gorm:"size:16" json:"target_type,omitempty"`
	TargetID   uint      `gorm:"index" json:"target_id"`
	JobID      string    `gorm:"size:64;index" json:"job_id,omitempty"`
	Status     string    `gorm:"size:16;default:pending;index" json:"status"`
	Result     string    `gorm:"size:16" json:"result,omitempty"`
	Label      string    `gorm:"size:64" json:"label,omitempty"`
	RawResp    string    `gorm:"type:text" json:"raw_resp,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
