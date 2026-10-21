package model

import "time"

// 点歌状态
type SongStatus string

const (
	SongPending  SongStatus = "pending"  // 待播放
	SongPlayed   SongStatus = "played"   // 已播放
	SongCanceled SongStatus = "canceled" // 已取消
)

// Song 点歌记录。
type Song struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index;index:idx_song_user_created,priority:1" json:"user_id"`
	Title     string     `gorm:"size:200" json:"title"`
	Artist    string     `gorm:"size:200" json:"artist"`
	Note      string     `gorm:"type:text" json:"note"`
	Status    SongStatus `gorm:"size:20;default:pending;index" json:"status"`
	CreatedAt time.Time  `gorm:"index;index:idx_song_user_created,priority:2" json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	Username string `gorm:"-" json:"username,omitempty"`
}
