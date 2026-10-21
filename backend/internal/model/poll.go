package model

import (
	"time"

	"gorm.io/gorm"
)

// Poll 投票。
type Poll struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Title       string         `gorm:"size:200" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	Multiple    bool           `gorm:"default:false" json:"multiple"`
	Pinned      bool           `gorm:"default:false;index;index:idx_poll_pinned_created,priority:1" json:"pinned"`
	Deadline    *time.Time     `json:"deadline"`
	CreatedAt   time.Time      `gorm:"index;index:idx_poll_pinned_created,priority:2" json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Username string       `gorm:"-" json:"username,omitempty"`
	Options  []PollOption `gorm:"foreignKey:PollID" json:"options,omitempty"`
}

// PollOption 投票选项。
type PollOption struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	PollID uint   `gorm:"index" json:"poll_id"`
	Text   string `gorm:"size:200" json:"text"`
	Sort   int    `gorm:"default:0" json:"sort"`
}

// PollVote 投票记录（同一用户对同一投票可改票，改票时覆盖旧票）。
type PollVote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PollID    uint      `gorm:"uniqueIndex:idx_poll_vote" json:"poll_id"`
	OptionID  uint      `gorm:"uniqueIndex:idx_poll_vote" json:"option_id"`
	UserID    uint      `gorm:"uniqueIndex:idx_poll_vote" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
