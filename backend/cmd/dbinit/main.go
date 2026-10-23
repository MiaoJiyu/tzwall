package main

import (
	"log/slog"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/database"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/permission"
)

// 一次性数据库初始化：执行迁移，并写入基础数据（超管、预选标签、角色权限集、协议），
// 但跳过系统配置（system_config）的初始化。
func main() {
	log := slog.Default()
	cfg := config.Load()
	db, err := database.Open(cfg, log)
	if err != nil {
		log.Error("打开数据库失败", "err", err.Error())
		os.Exit(1)
	}
	if err := database.Migrate(db, log); err != nil {
		log.Error("迁移失败", "err", err.Error())
		os.Exit(1)
	}
	if err := seedNoConfig(db, cfg, log); err != nil {
		log.Error("种子数据写入失败", "err", err.Error())
		os.Exit(1)
	}
	log.Info("数据库初始化完成（已跳过系统配置）")
}

func seedNoConfig(db *gorm.DB, cfg *config.Config, log *slog.Logger) error {
	// 初始超级管理员（仅当不存在时）
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

	// 预选标签（审核通过）
	for _, name := range []string{"校园生活", "学习交流", "失物招领", "表白墙", "吐槽", "求助", "活动通知", "二手交易"} {
		var c int64
		db.Model(&model.Tag{}).Where("name = ?", name).Count(&c)
		if c == 0 {
			if err := db.Create(&model.Tag{Name: name, Status: model.TagApproved, CreatedBy: 0}).Error; err != nil {
				return err
			}
		}
	}

	// 角色默认权限集（幂等）
	if err := permission.EnsureDefaults(db); err != nil {
		return err
	}

	// 协议占位文本（后台可编辑）
	for _, a := range []struct{ Key, Title, Content string }{
		{"user", "用户协议", "# 用户协议\n\n（占位文本，请在后台系统配置中编辑正式内容。）\n\n欢迎使用泰兴中学校园墙。使用本服务即表示你同意以下条款……"},
		{"privacy", "隐私协议", "# 隐私协议\n\n（占位文本，请在后台系统配置中编辑正式内容。）\n\n我们如何收集、使用与保护你的个人信息……"},
		{"data", "数据保护协议", "# 数据保护协议\n\n（占位文本，请在后台系统配置中编辑正式内容。）\n\n你的数据权利与留存策略……"},
	} {
		var c int64
		db.Model(&model.Agreement{}).Where("key = ?", a.Key).Count(&c)
		if c == 0 {
			if err := db.Create(&model.Agreement{Key: a.Key, Title: a.Title, Version: "1.0", Content: a.Content, EffectiveAt: time.Now()}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
