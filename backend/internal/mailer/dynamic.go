package mailer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// Settings 邮件发送所需配置快照（由调用方从系统配置或 .env 组装）。
type Settings struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// SettingsFunc 配置快照读取函数；每次发送时调用，实现保存即生效。
type SettingsFunc func() Settings

// Dynamic 按配置快照热重建的邮件发送器：签名变化即重建内部 SMTP 实例。
type Dynamic struct {
	fn   SettingsFunc
	mu   sync.Mutex
	sig  string
	base Mailer // 最近一次构建的发送器
}

// NewDynamic 构造动态邮件发送器。
func NewDynamic(fn SettingsFunc) *Dynamic { return &Dynamic{fn: fn} }

// Configured SMTP 是否已配置。
func (d *Dynamic) Configured() bool {
	s := d.fn()
	return s.Host != "" && s.User != ""
}

// Send 发送邮件（必要时重建发送器）。
func (d *Dynamic) Send(ctx context.Context, to, subject, body string) error {
	m, err := d.resolve()
	if err != nil {
		return err
	}
	return m.Send(ctx, to, subject, body)
}

// resolve 按当前配置返回发送器；签名一致则复用。
func (d *Dynamic) resolve() (Mailer, error) {
	s := d.fn()
	sig := signature(s)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.base != nil && d.sig == sig {
		return d.base, nil
	}
	if s.Host == "" || s.User == "" {
		return nil, fmt.Errorf("SMTP 未配置")
	}
	m := NewSMTPWithSettings(s)
	d.base = m
	d.sig = sig
	return m, nil
}

// signature 配置指纹：包含密码摘要，避免签名泄露明文。
func signature(s Settings) string {
	sum := sha256.Sum256([]byte(s.Pass))
	return fmt.Sprintf("%s|%d|%s|%s|%s", s.Host, s.Port, s.User, s.From, hex.EncodeToString(sum[:8]))
}
