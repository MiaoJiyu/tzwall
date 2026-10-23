package database

import (
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/permission"
)

// defaultConfigs 系统配置默认键值。仅在键不存在时插入，避免覆盖后台已改配置。
var defaultConfigs = map[string]string{
	"email_verify_ttl_hours":    "24",
	"email_resend_limit_hour":   "5",
	"login_fail_user_limit":     "5",
	"login_fail_admin_limit":    "10",
	"login_lock_minutes":        "15",
	"register_rph":              "20",
	"login_rph":                 "30",
	"miniprogram_name":          "泰兴中学校园墙",
	"miniprogram_login_desc":    "请使用微信扫描小程序码，在小程序内完成登录/注册。",
	"miniprogram_bind_desc":     "请使用微信扫描小程序码，在小程序内完成微信绑定。",
	"comment_max_len":           "500",
	"comment_max_depth":         "3",
	"tag_max_count":             "5",
	"image_max_count":           "9",
	"image_max_size_mb":         "5",
	"image_formats":             "jpg,jpeg,png,webp,gif",
	"video_max_size_mb":          "100",
	"video_max_count":            "3",
	"video_formats":              "mp4,mov,webm,m4v",
	"watermark_cache_seconds":   "60",
	"image_proxy_rpm":           "120",
	"log_access_retention_days": "90",
	"log_audit_retention_days":  "365",
	"report_hour_limit":         "5",
	"report_day_limit":          "20",
	"hot_decay_gravity":         "1.5",
	"footer_icp":                "",
	"footer_psba":               "",
	"footer_custom":             "",
	"agreement_required":        "true",
}

// defaultTags 预选标签（审核通过）。
var defaultTags = []string{"校园生活", "学习交流", "失物招领", "表白墙", "吐槽", "求助", "活动通知", "二手交易"}

// defaultAgreements 三份协议占位文本（后台可编辑）。
var defaultAgreements = []struct {
	Key, Title, Content string
}{
	{"user", "用户协议", "# 用户协议\n\n（占位文本，请在后台系统配置中编辑正式内容。）\n\n欢迎使用泰兴中学校园墙。使用本服务即表示你同意以下条款……"},
	{"privacy", "隐私协议", "# 隐私协议\n\n（占位文本，请在后台系统配置中编辑正式内容。）\n\n我们如何收集、使用与保护你的个人信息……"},
	{"data", "数据保护协议", "# 数据保护协议\n\n（占位文本，请在后台系统配置中编辑正式内容。）\n\n你的数据权利与留存策略……"},
}

// Seed 首次启动写入基础数据。
func Seed(db *gorm.DB, cfg *config.Config, log *slog.Logger) error {
	// 超管
	var adminCnt int64
	db.Model(&model.User{}).Where("username = ?", cfg.AdminUsername).Count(&adminCnt)
	if adminCnt == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		admin := &model.User{
			Username:         cfg.AdminUsername,
			Email:            cfg.AdminEmail,
			PasswordHash:     string(hash),
			RealName:         cfg.AdminRealName,
			School:           cfg.AdminSchool,
			Role:             model.RoleSuperAdmin,
			EmailVerified:    true,
			Status:           model.UserStatusActive,
			AgreementVersion: "1.0",
		}
		if err := db.Create(admin).Error; err != nil {
			return err
		}
		log.Info("已创建初始超级管理员", "username", cfg.AdminUsername)
	}

	// 系统配置
	for k, v := range defaultConfigs {
		var c int64
		db.Model(&model.SystemConfig{}).Where("key = ?", k).Count(&c)
		if c == 0 {
			if err := db.Create(&model.SystemConfig{Key: k, Value: v, UpdatedAt: time.Now()}).Error; err != nil {
				return err
			}
		}
	}

	// 预选标签
	for _, name := range defaultTags {
		var c int64
		db.Model(&model.Tag{}).Where("name = ?", name).Count(&c)
		if c == 0 {
			if err := db.Create(&model.Tag{Name: name, Status: model.TagApproved, CreatedBy: 0}).Error; err != nil {
				return err
			}
		}
	}

	// 角色默认权限集（按端点目录初始化，幂等）
	if err := permission.EnsureDefaults(db); err != nil {
		return err
	}

	// 协议
	for _, a := range defaultAgreements {
		var c int64
		db.Model(&model.Agreement{}).Where("key = ?", a.Key).Count(&c)
		if c == 0 {
			if err := db.Create(&model.Agreement{
				Key: a.Key, Title: a.Title, Version: "1.0",
				Content: a.Content, EffectiveAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
