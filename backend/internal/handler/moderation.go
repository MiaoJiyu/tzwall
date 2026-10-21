package handler

import (
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
)

// ModerationHandler 审核结果回调：兼容「我方 CI 提交（job_id 关联）」与
// 「COS 内容审核模块（按对象 Key）」两种回调格式。
type ModerationHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewModerationHandler(db *gorm.DB, cfg *config.Config) *ModerationHandler {
	return &ModerationHandler{db: db, cfg: cfg}
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
	if key, forbidden, ok := parseContentAudit(body); ok {
		if forbidden {
			h.blockImageByKey(key)
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

	// 联动业务对象：bizID 形如 sub-<投稿ID> 或 img-<对象键>
	switch {
	case strings.HasPrefix(task.BizID, "sub-"):
		if sid, err := strconv.ParseUint(strings.TrimPrefix(task.BizID, "sub-"), 10, 64); err == nil {
			h.db.Model(&model.Submission{}).Where("id = ?", uint(sid)).
				Update("audit_status", status)
			if v.Blocked {
				h.db.Model(&model.Submission{}).Where("id = ?", uint(sid)).
					Update("status", model.StatusPending)
			}
		}
	case strings.HasPrefix(task.BizID, "img-"):
		key := strings.TrimPrefix(task.BizID, "img-")
		updates := map[string]any{"audit_status": status}
		if v.Blocked {
			updates["is_private"] = true
		}
		h.db.Model(&model.SubmissionImage{}).Where("storage_key = ?", key).Updates(updates)
	}
}

// blockImageByKey 将指定对象键的图片标记为违规私有（停止对外展示）。
func (h *ModerationHandler) blockImageByKey(key string) {
	if key == "" {
		return
	}
	h.db.Model(&model.SubmissionImage{}).Where("storage_key = ?", key).
		Updates(map[string]any{"audit_status": model.ModerationBlock, "is_private": true})
}

// contentAudit 兼容数据万象「详细回调(DETAIL)」与「简洁回调(SIMPLE)」两种结构。
type contentAudit struct {
	EventName  string `json:"EventName"`
	JobsDetail struct {
		Object      string `json:"Object"`
		Result      int    `json:"Result"`
		ForbidState int    `json:"ForbidState"`
	} `json:"JobsDetail"`
	Code int `json:"code"`
	Data struct {
		URL             string `json:"url"`
		Result          int    `json:"result"`
		ForbiddenStatus int    `json:"forbidden_status"`
	} `json:"data"`
}

// parseContentAudit 解析内容审核回调，返回对象 Key 与是否违规/被冻结。
func parseContentAudit(body []byte) (key string, forbidden bool, ok bool) {
	var ca contentAudit
	if err := json.Unmarshal(body, &ca); err != nil {
		return "", false, false
	}
	// DETAIL：JobsDetail.Object（提交时用 Object 参数才有）
	if ca.JobsDetail.Object != "" {
		return ca.JobsDetail.Object,
			ca.JobsDetail.Result == 1 || ca.JobsDetail.ForbidState >= 1,
			true
	}
	// SIMPLE：data.url 需从 URL 解析 Key
	if ca.Data.URL != "" {
		k := keyFromURL(ca.Data.URL)
		if k == "" {
			return "", false, false
		}
		return k, ca.Data.Result == 1 || ca.Data.ForbiddenStatus >= 1, true
	}
	return "", false, false
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
