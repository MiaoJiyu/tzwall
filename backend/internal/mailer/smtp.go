package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
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
// 输出 RFC 2047 编码的主题/发件人名，补全 Date/Message-ID 等必要头，并以
// multipart/alternative 同时携带纯文本与 HTML 正文——这些缺失正是 QQ/163 等
// 邮箱将邮件判为可疑、隔离为“邮件已删除”的常见诱因。
func (m *smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	if !m.Configured() {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	from := m.from
	if from == "" {
		from = m.user
	}
	msg := buildMessage(from, to, subject, body)

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
	if _, err := w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}

// buildMessage 构造符合 RFC 5322 的邮件（multipart/alternative：纯文本 + HTML）。
func buildMessage(from, to, subject, htmlBody string) []byte {
	displayFrom := encodeAddress(from, "泰兴中学校园墙")
	msgID := fmt.Sprintf("<%s@%s>", randHex(12), messageIDHost(from))
	date := time.Now().Format(time.RFC1123Z)
	plain := htmlToText(htmlBody)
	boundary := "tzwall-" + randHex(16)

	var buf bytes.Buffer
	buf.WriteString("From: " + displayFrom + "\r\n")
	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("Subject: " + rfc2047(subject) + "\r\n")
	buf.WriteString("Date: " + date + "\r\n")
	buf.WriteString("Message-ID: " + msgID + "\r\n")
	buf.WriteString("Reply-To: " + from + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("X-Auto-Response-Suppress: All\r\n")
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(plain + "\r\n")
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody + "\r\n")
	buf.WriteString("--" + boundary + "--\r\n")
	return buf.Bytes()
}

// rfc2047 对非 ASCII 头字段做 Base64 编码（=?UTF-8?B?...?=），避免中文主题/发件人名乱码或被风控。
func rfc2047(s string) string {
	if isASCII(s) {
		return s
	}
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

// encodeAddress 生成带显示名的地址；显示名含非 ASCII 时自动 RFC 2047 编码。
func encodeAddress(addr, name string) string {
	if name == "" {
		return addr
	}
	return rfc2047(name) + " <" + addr + ">"
}

// messageIDHost 取发件域名作为 Message-ID 主机部分（缺省用 tzwall.local）。
func messageIDHost(from string) string {
	if i := strings.LastIndex(from, "@"); i >= 0 {
		return strings.Trim(strings.TrimSpace(from[i+1:]), "<> ")
	}
	return "tzwall.local"
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// htmlToText 极简 HTML→纯文本：换行标签转行、去标签、反转义，供 multipart 纯文本部分使用。
func htmlToText(html string) string {
	repl := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n", "</div>", "\n", "</li>", "\n", "</tr>", "\n", "</h1>", "\n", "</h2>", "\n", "</h3>", "\n")
	s := repl.Replace(html)
	tagRe := regexp.MustCompile(`<[^>]+>`)
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
