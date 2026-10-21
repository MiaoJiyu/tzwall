package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// LogHandler 访问日志与审计日志查询（仅超级管理员）。
type LogHandler struct {
	db    *gorm.DB
	store *service.ConfigStore
}

func NewLogHandler(db *gorm.DB, store *service.ConfigStore) *LogHandler {
	return &LogHandler{db: db, store: store}
}

// AccessLogView 访问日志输出（模型 json 标签为 "-"，此处独立 DTO）。
type AccessLogView struct {
	ID          uint      `json:"id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Query       string    `json:"query"`
	StatusCode  int       `json:"status_code"`
	DurationMs  int64     `json:"duration_ms"`
	IP          string    `json:"ip"`
	UA          string    `json:"ua"`
	UserID      uint      `json:"user_id"`
	Username    string    `json:"username"`
	Headers     string    `json:"headers"`
	BodyPreview string    `json:"body_preview"`
	BodyDigest  string    `json:"body_digest"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditLogView 审计日志输出。
type AuditLogView struct {
	ID         uint      `json:"id"`
	ActorID    uint      `json:"actor_id"`
	ActorName  string    `json:"actor_name"`
	Action     string    `json:"action"`
	ObjectType string    `json:"object_type"`
	ObjectID   string    `json:"object_id"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

// AccessLogs 访问日志列表。
func (h *LogHandler) AccessLogs(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}
	q, page, pageSize := h.accessQuery(c)
	var (
		items []model.AccessLog
		total int64
	)
	q.Model(&model.AccessLog{}).Count(&total)
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	// 批量解析用户名，避免 N+1
	ids := make([]uint, 0, len(items))
	seen := map[uint]bool{}
	for i := range items {
		if items[i].UserID != 0 && !seen[items[i].UserID] {
			seen[items[i].UserID] = true
			ids = append(ids, items[i].UserID)
		}
	}
	names := map[uint]string{}
	if len(ids) > 0 {
		var users []model.User
		h.db.Select("id", "username").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}
	out := make([]AccessLogView, 0, len(items))
	for i := range items {
		out = append(out, AccessLogView{
			ID:          items[i].ID,
			Method:      items[i].Method,
			Path:        items[i].Path,
			Query:       items[i].Query,
			StatusCode:  items[i].StatusCode,
			DurationMs:  items[i].DurationMs,
			IP:          items[i].IP,
			UA:          items[i].UA,
			UserID:      items[i].UserID,
			Username:    names[items[i].UserID],
			Headers:     items[i].Headers,
			BodyPreview: items[i].BodyPreview,
			BodyDigest:  items[i].BodyDigest,
			CreatedAt:   items[i].CreatedAt,
		})
	}
	response.OK(c, gin.H{"items": out, "total": total, "page": page, "page_size": pageSize})
}

// AuditLogs 审计日志列表。
func (h *LogHandler) AuditLogs(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	q := h.db.Model(&model.AuditLog{})
	if v := c.Query("action"); v != "" {
		q = q.Where("action = ?", v)
	}
	if v := c.Query("object_type"); v != "" {
		q = q.Where("object_type = ?", v)
	}
	if v := c.Query("actor_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("actor_id = ?", uint(n))
		}
	}
	from, to := timeRange(c, h.store)
	if !from.IsZero() {
		q = q.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("created_at <= ?", to)
	}
	var (
		items []model.AuditLog
		total int64
	)
	q.Count(&total)
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	ids := make([]uint, 0, len(items))
	seen := map[uint]bool{}
	for i := range items {
		if items[i].ActorID != 0 && !seen[items[i].ActorID] {
			seen[items[i].ActorID] = true
			ids = append(ids, items[i].ActorID)
		}
	}
	names := map[uint]string{}
	if len(ids) > 0 {
		var users []model.User
		h.db.Select("id", "username").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}
	out := make([]AuditLogView, 0, len(items))
	for i := range items {
		out = append(out, AuditLogView{
			ID:         items[i].ID,
			ActorID:    items[i].ActorID,
			ActorName:  names[items[i].ActorID],
			Action:     items[i].Action,
			ObjectType: items[i].ObjectType,
			ObjectID:   items[i].ObjectID,
			Detail:     items[i].Detail,
			CreatedAt:  items[i].CreatedAt,
		})
	}
	response.OK(c, gin.H{"items": out, "total": total, "page": page, "page_size": pageSize})
}

// accessQuery 组装访问日志查询条件。
func (h *LogHandler) accessQuery(c *gin.Context) (*gorm.DB, int, int) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	q := h.db.Model(&model.AccessLog{})
	if v := c.Query("path"); v != "" {
		q = q.Where("path ILIKE ?", "%"+v+"%")
	}
	if v := c.Query("method"); v != "" {
		q = q.Where("method = ?", v)
	}
	if v := c.Query("status"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q = q.Where("status_code = ?", n)
		}
	}
	if v := c.Query("user_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("user_id = ?", uint(n))
		}
	}
	if v := c.Query("ip"); v != "" {
		q = q.Where("ip = ?", v)
	}
	from, to := timeRange(c, h.store)
	if !from.IsZero() {
		q = q.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("created_at <= ?", to)
	}
	return q, page, pageSize
}

// timeRange 解析时间范围；缺省取最近 7 天，上限 90 天，避免全表扫描。
func timeRange(c *gin.Context, store *service.ConfigStore) (time.Time, time.Time) {
	now := time.Now()
	from := now.AddDate(0, 0, -7)
	to := now
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t.Add(24*time.Hour - time.Second)
		}
	}
	maxDays := 90
	if store != nil {
		if d := store.Int("log_access_retention_days", 90); d > 0 && d < maxDays {
			maxDays = d
		}
	}
	if to.Sub(from) > time.Duration(maxDays)*24*time.Hour {
		from = to.AddDate(0, 0, -maxDays)
	}
	return from, to
}
