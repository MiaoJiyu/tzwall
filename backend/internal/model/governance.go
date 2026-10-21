package model

import (
	"time"
)

// 处罚类型
type PunishmentType string

const (
	PunishWarn       PunishmentType = "warn"
	PunishMute       PunishmentType = "mute"
	PunishBanUser    PunishmentType = "ban_user"
	PunishBanIP      PunishmentType = "ban_ip"
	PunishForceReset PunishmentType = "force_reset"
)

// Punishment 处罚记录，可叠加、撤销仅标记不删。
type Punishment struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Type       PunishmentType `gorm:"size:20;index" json:"type"`
	Reason     string         `gorm:"type:text" json:"reason"`
	OperatorID uint           `json:"operator_id"`
	TargetIP   string         `gorm:"size:64" json:"target_ip,omitempty"`
	Permanent  bool           `gorm:"default:false" json:"permanent"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	Revoked    bool           `gorm:"default:false" json:"revoked"`
	RevokedAt  *time.Time     `json:"revoked_at,omitempty"`
	// AckAt 警告确认时间：非空表示该警告已被用户确认，不再弹窗
	AckAt *time.Time `json:"ack_at,omitempty"`
	// NewUsername 强制修改用户名后的新用户名（撤销不恢复）
	NewUsername string    `gorm:"size:64" json:"new_username,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// IPBan 独立 IP 封禁（可不限某用户），支持 IPv4/IPv6/CIDR。
type IPBan struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	CIDR       string     `gorm:"size:64;index" json:"cidr"`
	Reason     string     `gorm:"type:text" json:"reason"`
	OperatorID uint       `json:"operator_id"`
	Permanent  bool       `gorm:"default:false" json:"permanent"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Revoked    bool       `gorm:"default:false" json:"revoked"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Report 举报
type Report struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index" json:"user_id"`
	TargetType   string    `gorm:"size:16;index" json:"target_type"` // submission/comment
	TargetID     uint      `gorm:"index" json:"target_id"`
	Reason       string    `gorm:"type:text" json:"reason"`
	Status       string    `gorm:"size:20;default:pending;index" json:"status"`
	Result       string    `gorm:"size:20" json:"result,omitempty"`
	ResultReason string    `gorm:"type:text" json:"result_reason,omitempty"`
	AdminNote    string    `gorm:"type:text" json:"admin_note,omitempty"`
	HandledBy    uint      `json:"handled_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// PermissionGrant 端点/页面访问权限。
// UserID 为空表示角色默认权限集；非空表示针对单个用户的覆盖配置。
type PermissionGrant struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Role        string `gorm:"size:20;index" json:"role"` // public/user/admin
	UserID      *uint  `gorm:"index" json:"user_id,omitempty"`
	EndpointKey string `gorm:"size:80;index" json:"endpoint_key"`
	Allow       bool   `json:"allow"`
	Rps         int    `json:"rps"`
	Rpm         int    `json:"rpm"`
	RpHour      int    `json:"rphour"`
	RpDay       int    `json:"rpday"`
}
