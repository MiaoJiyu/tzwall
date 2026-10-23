package mailer

import (
	"context"
	"os"
	"testing"

	"tzwall/backend/internal/config"
)

// TestSendVerifyLinkTo163 同步发送一封真实验证邮件到 lucasoj@163.com，用于确认 SMTP 通道可用。
func TestSendVerifyLinkTo163(t *testing.T) {
	link := os.Getenv("VERIFY_LINK")
	if link == "" {
		t.Skip("VERIFY_LINK 为空，跳过")
	}
	cfg := config.Load()
	m := New(cfg)
	if !m.Configured() {
		t.Fatalf("邮件器未配置：host=%q user=%q", cfg.SmtpHost, cfg.SmtpUser)
	}
	body := `<div style="font-family:-apple-system,Segoe UI,Roboto,'PingFang SC',sans-serif;font-size:14px;color:#222;line-height:1.7">` +
		`<p style="margin:0 0 12px">请点击以下链接完成邮箱验证：</p>` +
		`<p style="margin:0 0 12px"><a href="` + link + `" style="color:#6b2a73;text-decoration:underline">` + link + `</a></p>` +
		`<p style="margin:0;color:#888;font-size:12px">如链接无法点击，请复制以下地址到浏览器打开：<br>` + link + `</p></div>`
	err := m.Send(context.Background(), "lucasoj@163.com", "【泰兴中学校园墙】邮箱验证", body)
	if err != nil {
		t.Fatalf("SMTP 发送失败: %v", err)
	}
	t.Logf("SMTP 发送成功 -> lucasoj@163.com；链接=%s", link)
}
