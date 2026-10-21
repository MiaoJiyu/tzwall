package ai

import "errors"

// Verdict AI 审核判定。
type Verdict struct {
	Pass   bool
	Reason string
	Raw    string
}

// 审核异常（区别于「审核不通过」）：异常一律转人工，绝不自动放行。
var (
	ErrNotConfigured = errors.New("AI 审核未配置")
	ErrQuotaExceeded = errors.New("AI 审核超出每日调用上限")
)
