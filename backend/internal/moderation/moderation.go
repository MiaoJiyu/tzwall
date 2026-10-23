package moderation

import (
	"context"

	"gorm.io/gorm"

	"tzwall/backend/internal/config"
)

// Verdict 审核判定结果。
type Verdict struct {
	Pass    bool
	Blocked bool
	Errored bool // 审核基础设施异常（如文件过大/解码失败/调用失败），非内容判定
	Reason  string
}

// Moderation 内容审核抽象。未配置或调用失败时转人工，绝不自动放行。
type Moderation interface {
	Configured() bool
	SubmitText(ctx context.Context, bizID, text string) (Verdict, error)
	SubmitImage(ctx context.Context, bizID, key string) (Verdict, error)
	QueryJob(ctx context.Context, jobID, kind string) (Verdict, error)
}

// New 选择审核实现：凭证完整时接入数据万象，否则降级直通（人工审核）。
func New(cfg *config.Config, db *gorm.DB) Moderation {
	if cfg.ModerationConfigured {
		if m := NewCOSModeration(cfg, db); m != nil {
			return m
		}
	}
	return NewNone()
}
