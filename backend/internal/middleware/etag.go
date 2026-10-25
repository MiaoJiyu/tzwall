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
//   - 未命中 → 正常返回，并附 ETag 与 private 短缓存（maxAge 秒）
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
		ew.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
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
