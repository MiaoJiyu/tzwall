package mailer

import (
	"context"

	"tzwall/backend/internal/config"
)

// Mailer 邮件发送抽象。未配置 SMTP 时降级为 noop（不发送，不报错）。
type Mailer interface {
	Configured() bool
	Send(ctx context.Context, to, subject, body string) error
}

// New 选择实现：.env 配置 SMTP 时用固定实现，否则交由 Dynamic 按后台配置热切换。
func New(cfg *config.Config) Mailer {
	if cfg.MailerConfigured {
		return NewSMTPWithSettings(Settings{
			Host: cfg.SmtpHost,
			Port: cfg.SmtpPort,
			User: cfg.SmtpUser,
			Pass: cfg.SmtpPass,
			From: cfg.SmtpFrom,
		})
	}
	return NewNoop()
}

// MaskPassword 脱敏展示 SMTP 密码。
func MaskPassword(p string) string {
	if len(p) <= 2 {
		return "****"
	}
	return p[:1] + "****" + p[len(p)-1:]
}
