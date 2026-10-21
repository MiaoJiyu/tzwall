package database

import (
	"log/slog"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
)

// Migrate 幂等迁移：建表 + pg_trgm 扩展与 GIN 索引（用于标题/正文/标签 LIKE 搜索）。
func Migrate(db *gorm.DB, log *slog.Logger) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		log.Warn("pg_trgm 扩展不可用，搜索降级为前缀匹配", "err", err.Error())
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Submission{},
		&model.Tag{},
		&model.SubmissionImage{},
		&model.SubmissionVideo{},
		&model.SubmissionRevision{},
		&model.Comment{},
		&model.Reaction{},
		&model.Agreement{},
		&model.AgreementVersion{},
		&model.WordFilter{},
		&model.SystemConfig{},
		&model.AccessLog{},
		&model.AuditLog{},
		&model.Punishment{},
		&model.IPBan{},
		&model.Report{},
		&model.PermissionGrant{},
		&model.ModerationTask{},
	); err != nil {
		return err
	}

	// 模糊搜索索引（扩展存在时生效）
	db.Exec("CREATE INDEX IF NOT EXISTS idx_submission_search ON submissions USING gin ((title || ' ' || content) gin_trgm_ops)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tag_search ON tags USING gin (name gin_trgm_ops)")
	// 列表排序复合索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_submission_status_created ON submissions (status, created_at DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_submission_user ON submissions (user_id, created_at DESC)")
	// 举报唯一性：同一用户对同一对象仅一次（并发兜底）
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_report_unique ON reports (user_id, target_type, target_id)")
	// 处罚查询
	db.Exec("CREATE INDEX IF NOT EXISTS idx_punishment_user ON punishments (user_id, created_at DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_ipban_active ON ip_bans (revoked, expires_at)")
	// 协议版本回查
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agreement_versions_key ON agreement_versions (key, created_at DESC)")
	return nil
}
