package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
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

// AccessLogger 访问日志：入口即记录 + 结果补全 + 断连看门狗，接近全量记录 + 脱敏 + 批量异步写入 + 保留期清理。
type AccessLogger struct {
	db      *gorm.DB
	store   *service.ConfigStore
	ch      chan *model.AccessLog
	pending sync.Map // key -> *pendingLog，处理中的请求（供看门狗识别被截断请求）
	dropped int64
	started bool
}

func NewAccessLogger(db *gorm.DB, store *service.ConfigStore) *AccessLogger {
	a := &AccessLogger{db: db, store: store, ch: make(chan *model.AccessLog, 8192)}
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
	// 看门狗：标记被客户端截断（超时断开）但 handler 未结束的请求
	go a.watchdog()
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
		// 通道满（高压/突发）：改为同步落库，避免丢记录——被截断的请求尤其不能被丢弃
		if a.db != nil {
			if err := a.db.Create(rec).Error; err != nil {
				atomic.AddInt64(&a.dropped, 1)
				slog.Default().Warn("访问日志写入失败", "err", err.Error())
			}
			return
		}
		atomic.AddInt64(&a.dropped, 1)
	}
}

// Log 访问日志中间件。
// 入口即建档：请求一到达就入队，**即使随后被客户端超时截断、或 handler 挂起，也已计入访问统计**；
// handler 正常结束时补全状态码/字节/缓存/资源/水印等结果字段。
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

		// 入口即建档（此时尚无状态码/字节数，StatusCode=0 表示"处理中"）
		rec := &model.AccessLog{
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path, // 记录真实路径（含 ID），便于审计追溯具体对象
			Query:       sanitize(truncate(c.Request.URL.RawQuery, 1024)),
			Headers:     pickHeaders(c),
			BodyPreview: bodyPreview,
			BodyDigest:  digest,
			UA:          truncate(c.Request.UserAgent(), 512),
			IP:          ClientIP(c),
			CreatedAt:   start,
		}
		a.enqueue(rec)
		key := fmt.Sprintf("%p", rec)
		a.pending.Store(key, &pendingLog{rec: rec, ctx: c.Request.Context()})
		defer func() {
			a.pending.Delete(key)
			a.finalize(rec, c, start)
		}()

		c.Next()
	}
}

// pendingLog 处理中的访问记录（供看门狗识别"客户端已断连但 handler 未结束"的请求）。
type pendingLog struct {
	rec *model.AccessLog
	ctx context.Context
}

// finalize 补全访问记录的结果字段。
// 若记录已被刷库（ID 已分配，说明请求较慢），补一次定向 UPDATE；尚未刷库则由 flusher 写最终值。
func (a *AccessLogger) finalize(rec *model.AccessLog, c *gin.Context, start time.Time) {
	if u := CurrentUser(c); u != nil {
		rec.UserID = u.ID
	}
	rec.StatusCode = c.Writer.Status()
	rec.BytesSent = int64(c.Writer.Size())
	rec.CacheStatus = c.GetString("cache_status")
	rec.Resource = c.GetString("resource")
	rec.Watermark = c.GetBool("watermark")
	rec.DurationMs = time.Since(start).Milliseconds()
	if rec.ID == 0 || a.db == nil {
		return
	}
	if err := a.db.Model(&model.AccessLog{}).Where("id = ?", rec.ID).Updates(map[string]any{
		"user_id":     rec.UserID,
		"status_code": rec.StatusCode,
		"bytes_sent":  rec.BytesSent,
		"cache_status": rec.CacheStatus,
		"resource":    rec.Resource,
		"watermark":   rec.Watermark,
		"duration_ms": rec.DurationMs,
	}).Error; err != nil {
		slog.Default().Warn("访问日志补全失败", "id", rec.ID, "err", err.Error())
	}
}

// watchdog 看门狗：客户端已断连（请求上下文被取消）而 handler 仍未结束的请求，
// 标记 Truncated 并（已入库时）补一次 UPDATE——保证压测超时截断的请求也在统计中可见。
func (a *AccessLogger) watchdog() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.pending.Range(func(k, v any) bool {
			pl, ok := v.(*pendingLog)
			if !ok || pl == nil || pl.rec == nil {
				a.pending.Delete(k)
				return true
			}
			// 客户端仍在正常等待，或刚断连（给 handler 一点收尾时间）→ 不标记
			if pl.ctx.Err() == nil || time.Since(pl.rec.CreatedAt) < 5*time.Second {
				return true
			}
			pl.rec.Truncated = true
			if pl.rec.ID != 0 && a.db != nil {
				_ = a.db.Model(&model.AccessLog{}).Where("id = ?", pl.rec.ID).Update("truncated", true).Error
			}
			return true
		})
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
