package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/storage"
)

// ModerationHandler 审核结果回调：兼容「我方 CI 提交（job_id 关联）」与
// 「COS 内容审核模块（按对象 Key）」两种回调格式。
type ModerationHandler struct {
	db      *gorm.DB
	cfg     *config.Config
	storage storage.Storage
}

func NewModerationHandler(db *gorm.DB, cfg *config.Config, st storage.Storage) *ModerationHandler {
	return &ModerationHandler{db: db, cfg: cfg, storage: st}
}

// Callback 处理审核结果回调（免鉴权）。
func (h *ModerationHandler) Callback(c *gin.Context) {
	if h.cfg.CosCallbackToken != "" {
		if c.GetHeader("X-Cos-Callback-Token") != h.cfg.CosCallbackToken {
			response.Fail(c, http.StatusUnauthorized, 401, "回调校验失败")
			return
		}
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, 400, "请求体读取失败")
		return
	}

	// 1) 我方提交并登记过的任务（数据万象 job_id 关联）
	jobID, v, parsed := moderation.ParseCallback(body)
	if parsed {
		var task model.ModerationTask
		if err := h.db.Where("job_id = ?", jobID).First(&task).Error; err == nil {
			h.applyTask(&task, v)
			response.OK(c, gin.H{"received": true})
			return
		}
	}

	// 2) COS 内容审核模块回调（控制台/自定义风险库），按对象 Key 处置
	if key, forbidden, label, ok := parseContentAudit(body); ok {
		if forbidden {
			h.blockMediaByKey(key, label)
		} else {
			h.passMediaByKey(key)
		}
		response.OK(c, gin.H{"received": true})
		return
	}

	if parsed {
		// 已解析但未登记任务：仅确认，不影响业务
		response.OK(c, gin.H{"received": true})
		return
	}
	response.Fail(c, http.StatusBadRequest, 400, "回调内容解析失败")
}

// applyTask 按任务 bizID 联动投稿/图片审核状态。
func (h *ModerationHandler) applyTask(task *model.ModerationTask, v moderation.Verdict) {
	status := moderation.StatusOf(v)
	h.db.Model(task).Updates(map[string]interface{}{
		"status":     status,
		"label":      v.Reason,
		"updated_at": time.Now(),
	})

	// 联动业务对象：bizID 形如 sub-<投稿ID>、img-<对象键>、vid-<对象键>
	switch {
	case strings.HasPrefix(task.BizID, "sub-"):
		if sid, err := strconv.ParseUint(strings.TrimPrefix(task.BizID, "sub-"), 10, 64); err == nil {
			if v.Blocked {
				// 腾讯云文本审核拒绝 → 立即驳回稿件
				h.rejectSubmission(h.db, uint(sid), "文本", v.Reason)
				return
			}
			h.db.Model(&model.Submission{}).Where("id = ?", uint(sid)).
				Update("audit_status", status)
		}
	case strings.HasPrefix(task.BizID, "img-"):
		key := strings.TrimPrefix(task.BizID, "img-")
		if v.Blocked {
			// 违规图片：下架图片并立即驳回所属投稿
			h.blockMediaByKey(key, v.Reason)
			return
		}
		var img model.SubmissionImage
		if err := h.db.Where("storage_key = ?", key).First(&img).Error; err == nil {
			h.db.Model(&model.SubmissionImage{}).Where("id = ?", img.ID).
				Update("audit_status", status)
			if img.SubmissionID != nil {
				h.reconcileSubmission(*img.SubmissionID)
			}
		}
	case strings.HasPrefix(task.BizID, "vid-"):
		key := strings.TrimPrefix(task.BizID, "vid-")
		if v.Blocked {
			h.blockMediaByKey(key, v.Reason)
			return
		}
		var vid model.SubmissionVideo
		if err := h.db.Where("storage_key = ?", key).First(&vid).Error; err == nil {
			h.db.Model(&model.SubmissionVideo{}).Where("id = ?", vid.ID).
				Update("audit_status", status)
			if vid.SubmissionID != nil {
				h.reconcileSubmission(*vid.SubmissionID)
			}
		}
	}
}

// blockMediaByKey 将指定对象键的图片或视频标记为违规私有（停止对外展示），
// 并立即驳回其所属投稿（机审已确认违规，无需再等人审）。
func (h *ModerationHandler) blockMediaByKey(key, label string) {
	if key == "" {
		return
	}
	_ = h.db.Transaction(func(tx *gorm.DB) error {
		var img model.SubmissionImage
		if err := tx.Where("storage_key = ?", key).First(&img).Error; err == nil {
			tx.Model(&model.SubmissionImage{}).Where("id = ?", img.ID).
				Updates(map[string]any{"audit_status": model.ModerationBlock, "is_private": true})
			if img.SubmissionID != nil {
				h.rejectSubmission(tx, *img.SubmissionID, "图片", label)
			}
			return nil
		}
		var vid model.SubmissionVideo
		if err := tx.Where("storage_key = ?", key).First(&vid).Error; err == nil {
			tx.Model(&model.SubmissionVideo{}).Where("id = ?", vid.ID).
				Updates(map[string]any{"audit_status": model.ModerationBlock, "is_private": true})
			if vid.SubmissionID != nil {
				h.rejectSubmission(tx, *vid.SubmissionID, "视频", label)
			}
			return nil
		}
		// 未登记的对象（非本站上传）无需处置
		return nil
	})
	// 清除源站缓存副本（避免 EdgeOne/本地仍提供已下架内容）
	if h.storage != nil {
		_ = h.storage.Purge(context.Background(), key)
	}
}

// rejectSubmission 机审确认违规后立即驳回投稿，驳回原因对作者可见。
func (h *ModerationHandler) rejectSubmission(tx *gorm.DB, submissionID uint, mediaKind, label string) {
	note := "内容未通过审核：" + mediaKind + "涉嫌违规"
	if l := strings.TrimSpace(label); l != "" {
		note += "（" + l + "）"
	}
	tx.Model(&model.Submission{}).Where("id = ?", submissionID).Updates(map[string]any{
		"status":       model.StatusRejected,
		"audit_status": model.ModerationBlock,
		"admin_note":   note,
	})
}

// passMediaByKey 媒体经腾讯云审核通过后，更新元数据状态并复核所属投稿是否可自动放行。
func (h *ModerationHandler) passMediaByKey(key string) {
	if key == "" {
		return
	}
	_ = h.db.Transaction(func(tx *gorm.DB) error {
		var img model.SubmissionImage
		if err := tx.Where("storage_key = ?", key).First(&img).Error; err == nil {
			tx.Model(&model.SubmissionImage{}).Where("id = ?", img.ID).
				Update("audit_status", model.ModerationPass)
			if img.SubmissionID != nil {
				h.reconcileSubmission(*img.SubmissionID)
			}
			return nil
		}
		var vid model.SubmissionVideo
		if err := tx.Where("storage_key = ?", key).First(&vid).Error; err == nil {
			tx.Model(&model.SubmissionVideo{}).Where("id = ?", vid.ID).
				Update("audit_status", model.ModerationPass)
			if vid.SubmissionID != nil {
				h.reconcileSubmission(*vid.SubmissionID)
			}
			return nil
		}
		return nil
	})
}

// reconcileSubmission 媒体异步审核完成后，若文本/AI 已通过且全部媒体均通过，则自动放行稿件。
func (h *ModerationHandler) reconcileSubmission(subID uint) {
	var sub model.Submission
	if h.db.First(&sub, subID).Error != nil {
		return
	}
	// 已被人工处置（驳回/通过/需修改）或仍处于待审，不自动放行
	if sub.Status == model.StatusRejected || sub.Status == model.StatusApproved ||
		sub.Status == model.StatusNeedEdit {
		return
	}
	// 文本/AI 机审未通过（block/failed/review）不自动放行，交由人工
	if sub.AuditStatus != model.ModerationPass {
		return
	}
	var badImgs, badVids int64
	h.db.Model(&model.SubmissionImage{}).
		Where("submission_id = ? AND audit_status = ?", subID, model.ModerationReview).Count(&badImgs)
	h.db.Model(&model.SubmissionVideo{}).
		Where("submission_id = ? AND audit_status = ?", subID, model.ModerationReview).Count(&badVids)
	if badImgs == 0 && badVids == 0 {
		h.db.Model(&model.Submission{}).Where("id = ?", subID).Update("status", model.StatusApproved)
	}
}

// contentAudit 兼容数据万象「详细回调(DETAIL)」与「简洁回调(SIMPLE)」两种结构。
type contentAudit struct {
	EventName  string `json:"EventName"`
	JobsDetail struct {
		Object      string `json:"Object"`
		Result      int    `json:"Result"`
		ForbidState int    `json:"ForbidState"`
		Label       string `json:"Label"`
	} `json:"JobsDetail"`
	Code int `json:"code"`
	Data struct {
		URL             string `json:"url"`
		Object          string `json:"object"`
		Result          int    `json:"result"`
		ForbiddenStatus int    `json:"forbidden_status"`
		Label           string `json:"label"`
	} `json:"data"`
}

// parseContentAudit 解析内容审核回调，返回对象 Key、是否违规/被冻结及违规标签。
func parseContentAudit(body []byte) (key string, forbidden bool, label string, ok bool) {
	var ca contentAudit
	if err := json.Unmarshal(body, &ca); err != nil {
		return "", false, "", false
	}
	// DETAIL：JobsDetail.Object（提交时用 Object 参数才有）
	if ca.JobsDetail.Object != "" {
		return ca.JobsDetail.Object,
			ca.JobsDetail.Result == 1 || ca.JobsDetail.ForbidState >= 1,
			ca.JobsDetail.Label,
			true
	}
	// SIMPLE：优先 data.object，缺失时从 data.url 解析 Key
	k := ca.Data.Object
	if k == "" && ca.Data.URL != "" {
		k = keyFromURL(ca.Data.URL)
	}
	if k == "" {
		return "", false, "", false
	}
	return k, ca.Data.Result == 1 || ca.Data.ForbiddenStatus >= 1, ca.Data.Label, true
}

// keyFromURL 从 COS 对象 URL 中剥离域名得到对象 Key。
func keyFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	p := strings.TrimPrefix(u.Path, "/")
	if dec, derr := url.PathUnescape(p); derr == nil {
		return dec
	}
	return p
}
