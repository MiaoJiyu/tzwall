package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/service"
)

// 敏感字段正则：密码、令牌、Cookie、密钥一律脱敏。
var sensitiveRe = regexp.MustCompile(`(?i)("(password|passwd|old_password|new_password|token|secret|secret_key|secret_id|authorization|cookie|smtp_pass|smtp_password|api_key)"\s*:\s*)"[^"]*"`)

// sanitize 将 JSON 中的敏感字段值替换为 ***。
func sanitize(s string) string {
	return sensitiveRe.ReplaceAllString(s, `${1}"***"`)
}

// AccessLogger 访问日志：接近全量记录 + 脱敏 + 批量异步写入 + 保留期清理。
type AccessLogger struct {
	db      *gorm.DB
	store   *service.ConfigStore
	ch      chan *model.AccessLog
	dropped int64
	started bool
}

func NewAccessLogger(db *gorm.DB, store *service.ConfigStore) *AccessLogger {
	a := &AccessLogger{db: db, store: store, ch: make(chan *model.AccessLog, 1024)}
	a.start()
	return a
}

func (a *AccessLogger) start() {
	if a.started {
		return
	}
	a.started = true
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		buf := make([]*model.AccessLog, 0, 50)
		for {
			select {
			case rec := <-a.ch:
				buf = append(buf, rec)
				if len(buf) >= 50 {
					a.flush(buf)
					buf = buf[:0]
				}
			case <-ticker.C:
				if len(buf) > 0 {
					a.flush(buf)
					buf = buf[:0]
				}
			}
		}
	}()
	// 保留期清理：启动时执行一次，之后每 24 小时
	a.cleanup()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			a.cleanup()
		}
	}()
}

func (a *AccessLogger) flush(buf []*model.AccessLog) {
	if len(buf) == 0 {
		return
	}
	if err := a.db.CreateInBatches(buf, len(buf)).Error; err != nil {
		slog.Default().Warn("访问日志批量写入失败", "count", len(buf), "err", err.Error())
	}
}

func (a *AccessLogger) cleanup() {
	if a.db == nil {
		return
	}
	days := 90
	auditDays := 365
	if a.store != nil {
		days = a.store.Int("log_access_retention_days", 90)
		auditDays = a.store.Int("log_audit_retention_days", 365)
	}
	accessCut := time.Now().AddDate(0, 0, -days)
	auditCut := time.Now().AddDate(0, 0, -auditDays)
	a.db.Where("created_at < ?", accessCut).Delete(&model.AccessLog{})
	a.db.Where("created_at < ?", auditCut).Delete(&model.AuditLog{})
}

func (a *AccessLogger) enqueue(rec *model.AccessLog) {
	select {
	case a.ch <- rec:
	default:
		// 队列满则丢弃，避免阻塞业务
		atomic.AddInt64(&a.dropped, 1)
	}
}

// Log 访问日志中间件。
func (a *AccessLogger) Log() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// JSON 请求体：读取后恢复，供 handler 正常解析
		var bodyPreview, digest string
		ct := c.Request.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/json") && c.Request.Body != nil {
			b, err := io.ReadAll(io.LimitReader(c.Request.Body, 65536))
			if err == nil && len(b) > 0 {
				sum := sha256.Sum256(b)
				digest = hex.EncodeToString(sum[:])
				bodyPreview = truncate(sanitize(string(b)), 4096)
				c.Request.Body = io.NopCloser(bytes.NewReader(b))
			}
		}

		c.Next()

		var uid uint
		if u := CurrentUser(c); u != nil {
			uid = u.ID
		}
		rec := &model.AccessLog{
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path, // 记录真实路径（含 ID），便于审计追溯具体对象
			Query:       sanitize(truncate(c.Request.URL.RawQuery, 1024)),
			Headers:     pickHeaders(c),
			BodyPreview: bodyPreview,
			BodyDigest:  digest,
			UA:          truncate(c.Request.UserAgent(), 512),
			IP:          ClientIP(c),
			UserID:      uid,
			StatusCode:  c.Writer.Status(),
			DurationMs:  time.Since(start).Milliseconds(),
			CreatedAt:   time.Now(),
		}
		a.enqueue(rec)
	}
}

// pickHeaders 采集关键请求头并脱敏。
func pickHeaders(c *gin.Context) string {
	want := []string{"Referer", "Accept", "X-Forwarded-For", "X-Request-Id", "Content-Type", "X-Real-Ip"}
	m := map[string]string{}
	for _, k := range want {
		if v := c.GetHeader(k); v != "" {
			m[k] = truncate(v, 256)
		}
	}
	// Authorization/Cookie 只记录是否存在，不记录内容
	if c.GetHeader("Authorization") != "" {
		m["Authorization"] = "***"
	}
	if c.GetHeader("Cookie") != "" {
		m["Cookie"] = "***"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
