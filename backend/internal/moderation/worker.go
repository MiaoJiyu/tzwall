package moderation

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
)

// Worker 审核结果轮询：回调不可用时兜底查询异步任务结果。
type Worker struct {
	db  *gorm.DB
	mod Moderation
	log *slog.Logger
}

func NewWorker(db *gorm.DB, mod Moderation, log *slog.Logger) *Worker {
	return &Worker{db: db, mod: mod, log: log}
}

// Start 启动轮询循环（每 60 秒一次）。
func (w *Worker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.poll()
			}
		}
	}()
}

func (w *Worker) poll() {
	if w.db == nil || w.mod == nil || !w.mod.Configured() {
		return
	}
	var tasks []model.ModerationTask
	w.db.Where("status = ? AND job_id <> ''", model.ModerationPending).Limit(50).Find(&tasks)
	for _, t := range tasks {
		v, err := w.mod.QueryJob(context.Background(), t.JobID, t.Kind)
		if err != nil {
			continue
		}
		status := statusOf(v)
		w.db.Model(&model.ModerationTask{}).Where("id = ?", t.ID).
			Updates(map[string]interface{}{"status": status, "label": v.Reason, "updated_at": time.Now()})
		if w.log != nil {
			w.log.Info("审核任务轮询更新", "job", t.JobID, "status", status)
		}
	}
}
