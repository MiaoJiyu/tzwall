package mailer

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBuildMessageBase64RoundTrip(t *testing.T) {
	html := `<p>请点击 <a href="https://tzwall.miaojiyu.top/verify?token=abc">链接</a> 完成验证</p>`
	msg := buildMessage("lucasoj@163.com", "user@qq.com", "【泰兴中学校园墙】邮箱验证", html)
	s := string(msg)
	if !strings.Contains(s, "multipart/alternative") {
		t.Fatal("missing multipart/alternative")
	}
	if !strings.Contains(s, "Content-Transfer-Encoding: base64") {
		t.Fatal("missing base64 transfer encoding")
	}
	if !strings.Contains(s, "Subject: =?UTF-8?B?") {
		t.Fatal("subject not RFC2047 encoded")
	}
	plain := htmlToText(html)
	dec, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(b64wrap(plain), "\r\n", ""))
	if err != nil {
		t.Fatalf("b64wrap not decodable: %v", err)
	}
	if string(dec) != plain {
		t.Fatalf("plain roundtrip mismatch: %q", string(dec))
	}
	dech, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(b64wrap(html), "\r\n", ""))
	if err != nil {
		t.Fatalf("b64wrap(html) not decodable: %v", err)
	}
	if string(dech) != html {
		t.Fatalf("html roundtrip mismatch: %q", string(dech))
	}
}
