package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// etagWriter 缓冲响应体，以便在写出前计算 ETag 并支持 304。
// 仅用于小体积 JSON 接口（meta/公告等），避免重复传输相同内容。
type etagWriter struct {
	gin.ResponseWriter
	body   []byte
	status int
}

func (w *etagWriter) WriteHeader(code int) {
	w.status = code // 延迟到中间件末尾统一写出
}

func (w *etagWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *etagWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush 透传（SSE 等场景不使用本中间件）。
func (w *etagWriter) Flush() { w.ResponseWriter.Flush() }

// ETag 为 GET JSON 响应生成弱 ETag：
//   - 命中 If-None-Match → 304（不发body）
//   - 未命中 → 正常返回，并附 ETag 与 private 缓存策略
//
// maxAge 语义：
//   - maxAge >= 0：允许浏览器在 maxAge 秒内直接使用本地缓存（适用于全局、不因用户/动作变化的资源，如 meta 配置）
//   - maxAge <  0：no-cache —— 浏览器每次都带 If-None-Match 校验，未变 304、变了 200。
//     适用于含「按用户变化」字段的响应（如公告列表的 confirmed，会因用户点击确认而改变），
//     避免用户确认后刷新仍读到过期的 confirmed:false。
//
// 用于降低 meta/公告等高频小接口的重复公网上行。
func ETag(maxAge int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		ew := &etagWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = ew
		c.Next()

		ct := ew.Header().Get("Content-Type")
		if ew.status != http.StatusOK || !strings.Contains(ct, "application/json") || len(ew.body) == 0 {
			// 非 2xx JSON：按原样透传
			ew.ResponseWriter.WriteHeader(ew.status)
			_, _ = ew.ResponseWriter.Write(ew.body)
			return
		}
		sum := sha256.Sum256(ew.body)
		etag := fmt.Sprintf(`W/"%s"`, hex.EncodeToString(sum[:])[:16])
		ew.Header().Set("ETag", etag)
		if maxAge < 0 {
			// 含按用户变化字段，必须每次校验，禁止直接使用本地缓存
			ew.Header().Set("Cache-Control", "private, no-cache")
		} else {
			ew.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
		}
		if match := c.Request.Header.Get("If-None-Match"); match != "" && etagMatch(match, etag) {
			ew.ResponseWriter.WriteHeader(http.StatusNotModified)
			return
		}
		ew.ResponseWriter.WriteHeader(http.StatusOK)
		_, _ = ew.ResponseWriter.Write(ew.body)
	}
}

// etagMatch 比较 If-None-Match（支持弱比较与 * 通配）。
func etagMatch(header, etag string) bool {
	if strings.TrimSpace(header) == "*" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimSpace(part)
		if p == etag || strings.TrimPrefix(p, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}
