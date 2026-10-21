package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// ReportHandler 举报相关接口。
type ReportHandler struct {
	svc        *service.ReportService
	subSvc     *service.SubmissionService
	commentSvc *service.CommentService
}

func NewReportHandler(svc *service.ReportService, subSvc *service.SubmissionService, commentSvc *service.CommentService) *ReportHandler {
	return &ReportHandler{svc: svc, subSvc: subSvc, commentSvc: commentSvc}
}

// TargetDetail 查看被举报内容（投稿含评论），供举报处理时快捷预览。
func (h *ReportHandler) TargetDetail(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	r, err := h.svc.Get(uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	var subID uint
	focus := uint(0)
	if r.TargetType == "comment" {
		cm, cerr := h.commentSvc.Get(r.TargetID)
		if cerr != nil {
			code, _ := errs.As(cerr)
			response.FailCode(c, code, cerr.Error())
			return
		}
		subID = cm.SubmissionID
		focus = cm.ID
	} else {
		subID = r.TargetID
	}
	sub, serr := h.subSvc.GetVisible(subID, actor)
	if serr != nil {
		code, _ := errs.As(serr)
		response.FailCode(c, code, serr.Error())
		return
	}
	comments, _ := h.commentSvc.List(subID, true)
	response.OK(c, gin.H{
		"target_type":      r.TargetType,
		"submission":       submissionView(sub, actor),
		"comments":         comments,
		"focus_comment_id": focus,
	})
}

// Create 提交举报（必须登录）。
func (h *ReportHandler) Create(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var req struct {
		TargetType string `json:"target_type"`
		TargetID   uint   `json:"target_id"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Create(u, req.TargetType, req.TargetID, req.Reason); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Mine 我的举报（可查看处理结果）。
func (h *ReportHandler) Mine(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	rows, err := h.svc.Mine(u.ID)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// AdminList 举报列表（管理员）。
func (h *ReportHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	rows, total, err := h.svc.ListForAdmin(c.Query("status"), c.Query("target_type"), page, pageSize)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize})
}

// Resolve 处理举报：撤下内容 / 无违规 / 处罚用户。
func (h *ReportHandler) Resolve(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Result       string               `json:"result"`
		Reason       string               `json:"reason"`
		Note         string               `json:"note"`
		PunishType   model.PunishmentType `json:"punish_type"`
		PunishTarget string               `json:"punish_target"`
		PunishReason string               `json:"punish_reason"`
		Permanent    bool                 `json:"permanent"`
		ExpiresAt    *string              `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if req.Result != service.ReportResultDismiss && req.Result != service.ReportResultRemove {
		response.FailCode(c, errs.ErrValidation, "处理结果不合法")
		return
	}
	if err := h.svc.Resolve(actor, uint(id), service.ResolveRequest{
		Result:       req.Result,
		Reason:       req.Reason,
		Note:         req.Note,
		PunishType:   req.PunishType,
		PunishTarget: req.PunishTarget,
		PunishReason: req.PunishReason,
		Permanent:    req.Permanent,
		ExpiresAt:    parseTimePtr(req.ExpiresAt),
	}); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
