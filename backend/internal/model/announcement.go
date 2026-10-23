package model

import "time"

// Announcement 校园公告（后台发布/修改/删除，可要求全员确认）。
type Announcement struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Title          string    `gorm:"size:200" json:"title"`
	Content        string    `gorm:"type:text" json:"content"`
	RequireConfirm bool      `gorm:"default:false" json:"require_confirm"` // 是否要求全员确认
	Pinned         bool      `gorm:"default:false" json:"pinned"`         // 置顶
	Active         bool      `gorm:"default:true" json:"active"`          // 是否上架（发布）
	CreatedBy      uint      `gorm:"index" json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AnnouncementConfirm 用户对“要求确认”公告的确认记录（按用户去重）。
type AnnouncementConfirm struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	AnnouncementID uint      `gorm:"uniqueIndex:idx_ann_confirm" json:"announcement_id"`
	UserID         uint      `gorm:"uniqueIndex:idx_ann_confirm" json:"user_id"`
	CreatedAt      time.Time `json:"created_at"`
}
