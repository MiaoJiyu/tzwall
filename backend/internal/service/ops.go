package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/storage"
)

// 健康项状态
const (
	HealthOK   = "ok"
	HealthWarn = "warn"
	HealthDown = "down"
	HealthOff  = "off"
)

// BackupFile 备份文件信息。
type BackupFile struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HealthItem 单个依赖健康状态。
type HealthItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// OpsOverview 运维面板聚合数据。
type OpsOverview struct {
	Uploads struct {
		Path  string `json:"path"`
		Bytes int64  `json:"bytes"`
	} `json:"uploads"`
	Disk struct {
		Total       uint64  `json:"total"`
		Free        uint64  `json:"free"`
		UsedPercent float64 `json:"used_percent"`
		Path        string  `json:"path"`
	} `json:"disk"`
	Database struct {
		Bytes int64 `json:"bytes"`
	} `json:"database"`
	COS struct {
		Objects  int64 `json:"objects"`
		Bytes    int64 `json:"bytes"`
		AlertGB  int64 `json:"alert_gb"`
		Exceeded bool  `json:"exceeded"`
	} `json:"cos"`
	Backups struct {
		Dir       string       `json:"dir"`
		Files     []BackupFile `json:"files"`
		LastRunAt *time.Time   `json:"last_run_at"`
	} `json:"backups"`
	Health []HealthItem `json:"health"`
	Alerts []string     `json:"alerts"`
}

// OpsService 运维数据采集（带 60 秒缓存，避免每次请求都统计磁盘）。
type OpsService struct {
	db         *gorm.DB
	cfg        *config.Config
	store      *ConfigStore
	c          cache.Cache
	moderation moderation.Moderation
	storage    storage.Storage
	aiReady    func() bool

	mu       sync.Mutex
	cached   *OpsOverview
	cachedAt time.Time
}

// NewOps 构造运维服务。aiReady 返回 AI 审核是否已启用且配置完整。
func NewOps(db *gorm.DB, cfg *config.Config, store *ConfigStore, c cache.Cache, mod moderation.Moderation, st storage.Storage, aiReady func() bool) *OpsService {
	return &OpsService{db: db, cfg: cfg, store: store, c: c, moderation: mod, storage: st, aiReady: aiReady}
}

// Overview 聚合运维数据。
func (s *OpsService) Overview(ctx context.Context) (*OpsOverview, error) {
	s.mu.Lock()
	if s.cached != nil && time.Since(s.cachedAt) < 60*time.Second {
		out := s.cached
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	o := &OpsOverview{}
	s.fillUploads(o)
	s.fillDisk(o)
	s.fillDatabase(o)
	s.fillCOS(o)
	s.fillBackups(o)
	s.fillHealth(ctx, o)

	s.mu.Lock()
	s.cached = o
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return o, nil
}

// fillUploads 统计本地上传目录体积。
func (s *OpsService) fillUploads(o *OpsOverview) {
	dir := s.store.Get("upload_dir")
	if dir == "" {
		dir = "./data/uploads"
	}
	o.Uploads.Path = dir
	var total int64
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			total += info.Size()
		}
		return nil
	})
	o.Uploads.Bytes = total
}

// fillDisk 统计磁盘总量与可用（以上传目录所在文件系统为准）。
func (s *OpsService) fillDisk(o *OpsOverview) {
	path := s.store.Get("upload_dir")
	if path == "" {
		path = "./data/uploads"
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	o.Disk.Path = path
	o.Disk.Total = total
	o.Disk.Free = free
	if total > 0 {
		o.Disk.UsedPercent = float64(total-free) / float64(total) * 100
	}
}

// fillDatabase 统计数据库体积。
func (s *OpsService) fillDatabase(o *OpsOverview) {
	var size int64
	if err := s.db.Raw("SELECT pg_database_size(current_database())").Scan(&size).Error; err == nil {
		o.Database.Bytes = size
	}
}

// fillCOS 以已上传图片元数据估算 COS 用量。
func (s *OpsService) fillCOS(o *OpsOverview) {
	var agg struct {
		Count int64
		Bytes int64
	}
	s.db.Model(&model.SubmissionImage{}).
		Select("count(*) as count, coalesce(sum(size),0) as bytes").Scan(&agg)
	var vAgg struct {
		Count int64
		Bytes int64
	}
	s.db.Model(&model.SubmissionVideo{}).
		Select("count(*) as count, coalesce(sum(size),0) as bytes").Scan(&vAgg)
	o.COS.Objects = agg.Count + vAgg.Count
	o.COS.Bytes = agg.Bytes + vAgg.Bytes
	o.COS.AlertGB = int64(s.store.Int("ops_cos_alert_gb", 50))
	o.COS.Exceeded = o.COS.Bytes > o.COS.AlertGB*(1<<30)
}

// fillBackups 读取备份目录。
func (s *OpsService) fillBackups(o *OpsOverview) {
	dir := s.store.Get("backup_dir")
	if dir == "" {
		dir = "/var/backups/tzwall"
	}
	o.Backups.Dir = dir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	files := make([]BackupFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		files = append(files, BackupFile{Name: e.Name(), Size: info.Size(), UpdatedAt: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].UpdatedAt.After(files[j].UpdatedAt) })
	if len(files) > 20 {
		files = files[:20]
	}
	o.Backups.Files = files
	if len(files) > 0 {
		last := files[0].UpdatedAt
		o.Backups.LastRunAt = &last
	}
}

// fillHealth 依赖健康检查。
func (s *OpsService) fillHealth(ctx context.Context, o *OpsOverview) {
	items := make([]HealthItem, 0, 6)

	// 数据库
	dbItem := HealthItem{Key: "db", Label: "PostgreSQL", Status: HealthOK, Detail: "连接正常"}
	if sql, err := s.db.DB(); err != nil {
		dbItem = HealthItem{Key: "db", Label: "PostgreSQL", Status: HealthDown, Detail: err.Error()}
	} else if err := sql.PingContext(ctx); err != nil {
		dbItem = HealthItem{Key: "db", Label: "PostgreSQL", Status: HealthDown, Detail: err.Error()}
	}
	items = append(items, dbItem)

	// Redis
	if s.cfg.RedisConfigured {
		items = append(items, HealthItem{Key: "redis", Label: "Redis", Status: HealthOK, Detail: s.cfg.RedisAddr})
	} else {
		items = append(items, HealthItem{Key: "redis", Label: "Redis", Status: HealthOff, Detail: "未配置（限流降级为内存）"})
	}

	// COS 存储（含连通性探测）
	if s.storage != nil && s.storage.Configured() {
		item := HealthItem{Key: "cos", Label: "COS 存储", Status: HealthOK, Detail: s.cfg.CosBucket + " / " + s.cfg.CosRegion}
		if err := s.storage.Probe(ctx); err != nil {
			item.Status = HealthWarn
			item.Detail = "连通性异常：" + truncateErr(err)
		}
		items = append(items, item)
	} else {
		items = append(items, HealthItem{Key: "cos", Label: "COS 存储", Status: HealthWarn, Detail: "未配置，图片存本地磁盘"})
	}

	// 内容审核（数据万象）
	if s.moderation != nil && s.moderation.Configured() {
		items = append(items, HealthItem{Key: "moderation", Label: "内容审核", Status: HealthOK, Detail: "数据万象已接入"})
	} else {
		items = append(items, HealthItem{Key: "moderation", Label: "内容审核", Status: HealthWarn, Detail: "未配置，全部转人工"})
	}

	// AI 审核
	if s.aiReady != nil && s.aiReady() {
		items = append(items, HealthItem{Key: "ai", Label: "AI 审核", Status: HealthOK, Detail: "已启用"})
	} else {
		items = append(items, HealthItem{Key: "ai", Label: "AI 审核", Status: HealthOff, Detail: "未启用，云审核通过后转人工"})
	}

	// 邮件
	if s.cfg.MailerConfigured {
		items = append(items, HealthItem{Key: "mailer", Label: "邮件服务", Status: HealthOK, Detail: s.cfg.SmtpHost})
	} else {
		items = append(items, HealthItem{Key: "mailer", Label: "邮件服务", Status: HealthOff, Detail: "未配置，注册免激活"})
	}

	o.Health = items

	// 告警汇总
	warnPercent := float64(s.store.Int("ops_disk_warn_percent", 85))
	if o.Disk.UsedPercent >= warnPercent {
		o.Alerts = append(o.Alerts, "磁盘使用率已达 "+formatPercent(o.Disk.UsedPercent))
	}
	if o.COS.Exceeded {
		o.Alerts = append(o.Alerts, "COS 估算用量已超过告警阈值")
	}
	if dbItem.Status == HealthDown {
		o.Alerts = append(o.Alerts, "数据库连接异常")
	}
}

// truncateErr 截断错误信息，避免面板出现过长文本。
func truncateErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 120 {
		return s[:120]
	}
	return s
}

// formatPercent 百分比格式化（保留一位小数）。
func formatPercent(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}
