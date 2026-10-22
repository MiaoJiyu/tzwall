package model

import (
	"time"

	"gorm.io/gorm"
)

// 角色
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleUser       Role = "user"
)

// 账号状态
const (
	UserStatusActive  = "active"
	UserStatusBanned  = "banned"  // 封禁用户名，无法登录
	UserStatusDeleted = "deleted" // 注销匿名化
)

// User 站点用户。真实姓名、就读学校仅后台可见。
type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;size:64" json:"username"`
	Email        string `gorm:"uniqueIndex;size:255" json:"email"`
	PasswordHash string `gorm:"size:100" json:"-"`
	RealName     string `gorm:"size:64" json:"-"`  // 仅后台
	School       string `gorm:"size:128" json:"-"` // 仅后台
	Wechat       string `gorm:"size:64" json:"wechat,omitempty"`
	QQ           string `gorm:"size:32" json:"qq,omitempty"`
	Phone        string `gorm:"size:32" json:"phone,omitempty"`

	// 第三方登录身份（与上面的 Wechat/QQ 自由文本联系方式区分）：
	// openid 按应用 appid 维度；unionid 为微信/QQ 开放平台跨应用统一身份，是账号关联的主键。
	WxOpenid  string `gorm:"size:64;index" json:"-"`
	WxUnionid string `gorm:"size:64;index" json:"-"`
	QqOpenid  string `gorm:"size:64;index" json:"-"`
	QqUnionid string `gorm:"size:64;index" json:"-"`

	// 社交资料（展示用，非敏感）
	Nickname string `gorm:"size:64" json:"nickname,omitempty"`
	Avatar   string `gorm:"size:512" json:"avatar,omitempty"`

	Role Role `gorm:"size:20;default:user;index" json:"role"`

	EmailVerified     bool       `gorm:"default:false" json:"email_verified"`
	EmailVerifyToken  string     `gorm:"size:64" json:"-"`
	EmailVerifyExpiry *time.Time `json:"-"`

	Status        string     `gorm:"size:20;default:active" json:"status"`
	LockedUntil   *time.Time `json:"-"`
	LoginFails    int        `json:"-"`
	TokenVersion  int64      `gorm:"default:1" json:"-"`
	ForceResetPwd bool       `gorm:"default:false" json:"-"`

	AnonName string `gorm:"size:64" json:"-"` // 注销后匿名化用户名

	AgreementVersion string `gorm:"size:32" json:"agreement_version,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// IsSuperAdmin / IsAdmin 角色判断
func (u *User) IsSuperAdmin() bool { return u.Role == RoleSuperAdmin }
func (u *User) IsAdmin() bool      { return u.Role == RoleAdmin || u.Role == RoleSuperAdmin }
func (u *User) IsBanned() bool     { return u.Status == UserStatusBanned }
