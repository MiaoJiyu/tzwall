package model

import "time"

// Conversation 两个用户之间的一对一会话（UserLow/UserHigh 规范排序后唯一）。
type Conversation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserLow   uint      `gorm:"index;uniqueIndex:idx_conv_pair" json:"user_low"`
	UserHigh  uint      `gorm:"uniqueIndex:idx_conv_pair" json:"user_high"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 瞬态（列表装配）
	Other       *User      `gorm:"-" json:"other,omitempty"`
	LastMessage string     `gorm:"-" json:"last_message,omitempty"`
	LastAt      *time.Time `gorm:"-" json:"last_at,omitempty"`
	Unread      int        `gorm:"-" json:"unread"`
}

// Message 私聊消息。
type Message struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID uint      `gorm:"index" json:"conversation_id"`
	SenderID       uint      `gorm:"index" json:"sender_id"`
	Content        string    `gorm:"type:text" json:"content"`
	Read           bool      `gorm:"default:false" json:"read"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`

	Mine bool `gorm:"-" json:"mine"` // 相对当前查看者
}
