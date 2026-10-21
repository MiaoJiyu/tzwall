package mailer

import (
	"context"
	"log/slog"
)

// noopMailer 未配置 SMTP 时降级：不发送，仅记录（不报错）。
type noopMailer struct {
	log *slog.Logger
}

func NewNoop() *noopMailer {
	return &noopMailer{log: slog.Default()}
}

func (m *noopMailer) Configured() bool { return false }

func (m *noopMailer) Send(ctx context.Context, to, subject, body string) error {
	if m.log != nil {
		m.log.Info("SMTP 未配置，邮件降级为不发送", "to", to, "subject", subject)
	}
	return nil
}
