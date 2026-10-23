package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func ctxWithURL(u string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", u, nil)
	return c
}

func TestImageViewerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initImageLinks("unit-secret", nil)

	// uid>0 → 带签名，可解析回 uid
	url := signedImageURL(42, 42)
	if url == "/api/images/42" {
		t.Fatal("uid>0 应返回带签名 URL")
	}
	if got := viewerUIDFromToken(ctxWithURL(url)); got != 42 {
		t.Fatalf("有效令牌 uid=%d want 42", got)
	}

	// uid=0 → 纯路径（游客）
	if raw := signedImageURL(42, 0); raw != "/api/images/42" {
		t.Fatalf("游客 URL=%q want /api/images/42", raw)
	}
	if got := viewerUIDFromToken(ctxWithURL("/api/images/42")); got != 0 {
		t.Fatalf("无令牌应 0，got %d", got)
	}

	// 篡改签名 → 0
	if got := viewerUIDFromToken(ctxWithURL("/api/images/42?u=42&e=9999999999&s=deadbeef")); got != 0 {
		t.Fatalf("篡改签名应 0，got %d", got)
	}
	// 过期 → 0
	if got := viewerUIDFromToken(ctxWithURL("/api/images/42?u=42&e=1&s=x")); got != 0 {
		t.Fatalf("过期应 0，got %d", got)
	}

	// 解析：新格式 / 游客 / 旧格式
	if u, ip, ts := parseWatermarkText("u~Jiyu~i~2abc~t~l3x"); u != "Jiyu" || ip != "2abc" || ts != "l3x" {
		t.Fatalf("新格式解析 u=%q ip=%q ts=%q", u, ip, ts)
	}
	if u, _, _ := parseWatermarkText("u~游客~i~2abc~t~l3x"); u != "游客" {
		t.Fatalf("游客解析 u=%q", u)
	}
	if u, ip, ts := parseWatermarkText("i~2abc~t~l3x"); u != "" || ip != "2abc" || ts != "l3x" {
		t.Fatalf("旧格式解析 u=%q ip=%q ts=%q", u, ip, ts)
	}
}
