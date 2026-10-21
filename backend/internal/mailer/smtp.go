package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// smtpMailer 基于标准库 net/smtp 的实现，支持显式/隐式 TLS 与超时。
type smtpMailer struct {
	host string
	port int
	user string
	pass string
	from string
}

// NewSMTPWithSettings 按配置快照构造 SMTP 发送器（供 Dynamic 热重建使用）。
func NewSMTPWithSettings(s Settings) *smtpMailer {
	return &smtpMailer{host: s.Host, port: s.Port, user: s.User, pass: s.Pass, from: s.From}
}

func (m *smtpMailer) Configured() bool { return m.host != "" && m.user != "" }

// Send 发送邮件。端口 465 走隐式 TLS；其余端口优先 STARTTLS。
// 通过 dialer 超时避免 SMTP 不可达时请求长时间挂起（issue 10）。
func (m *smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	if !m.Configured() {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	from := m.from
	if from == "" {
		from = m.user
	}
	msg := strings.Builder{}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	auth := smtp.PlainAuth("", m.user, m.pass, m.host)

	// 超时：优先取 ctx 截止时间，否则默认 10s
	timeout := 10 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 {
			timeout = d
		}
	}
	dialer := &net.Dialer{Timeout: timeout}

	var c *smtp.Client
	var err error
	if m.port == 465 {
		conn, derr := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: m.host,
			MinVersion: tls.VersionTLS12,
		})
		if derr != nil {
			return derr
		}
		c, err = smtp.NewClient(conn, m.host)
	} else {
		conn, derr := dialer.Dial("tcp", addr)
		if derr != nil {
			return derr
		}
		c, err = smtp.NewClient(conn, m.host)
		if err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if terr := c.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); terr != nil {
				return terr
			}
		}
	}
	if err != nil {
		return err
	}
	defer c.Quit()

	if err := c.Auth(auth); err != nil {
		return err
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return err
	}
	return w.Close()
}
