package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// PollHandler 投票接口。
type PollHandler struct {
	svc *service.PollService
}

func NewPollHandler(svc *service.PollService) *PollHandler {
	return &PollHandler{svc: svc}
}

type pollBody struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Multiple    bool     `json:"multiple"`
	Pinned      bool     `json:"pinned"`
	Deadline    *string  `json:"deadline"`
	Options     []string `json:"options"`
}

func (b pollBody) toReq() service.PollCreateReq {
	return service.PollCreateReq{
		Title:       b.Title,
		Description: b.Description,
		Multiple:    b.Multiple,
		Pinned:      b.Pinned,
		Deadline:    parseTimePtr(b.Deadline),
		Options:     b.Options,
	}
}

// List 公开投票列表。
func (h *PollHandler) List(c *gin.Context) {
	uid := uint(0)
	if u := middleware.CurrentUser(c); u != nil {
		uid = u.ID
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	items, total, err := h.svc.List(uid, page, pageSize, c.Query("keyword"))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// Detail 投票详情。
func (h *PollHandler) Detail(c *gin.Context) {
	uid := uint(0)
	if u := middleware.CurrentUser(c); u != nil {
		uid = u.ID
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	view, err := h.svc.Detail(uid, uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, view)
}

// Create 用户发起普通投票（不可置顶）。
func (h *PollHandler) Create(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var body pollBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	poll, err := h.svc.Create(u, body.toReq(), false)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"id": poll.ID})
}

// Vote 投票 / 改票。
func (h *PollHandler) Vote(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		OptionIDs []uint `json:"option_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Vote(u.ID, uint(id), req.OptionIDs); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// DeleteOwn 删除自己的投票。
func (h *PollHandler) DeleteOwn(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteOwn(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminList 后台投票列表。
func (h *PollHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	items, total, err := h.svc.List(0, page, pageSize, c.Query("keyword"))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminCreate 管理员发起投票（可选置顶高亮）。
func (h *PollHandler) AdminCreate(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	var body pollBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	poll, err := h.svc.Create(u, body.toReq(), true)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"id": poll.ID})
}

// AdminDelete 管理员强删投票。
func (h *PollHandler) AdminDelete(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.AdminDelete(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// SubmissionPolls 获取某投稿关联的投票（公开）。
func (h *PollHandler) SubmissionPolls(c *gin.Context) {
	uid := uint(0)
	if u := middleware.CurrentUser(c); u != nil {
		uid = u.ID
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	items, err := h.svc.SubmissionPolls(uint(id), uid)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "max": h.svc.MaxPollsPerSubmission()})
}

// SetSubmissionPolls 设置投稿关联的投票（作者或管理员）。
func (h *PollHandler) SetSubmissionPolls(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		PollIDs []uint `json:"poll_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.SetSubmissionPolls(u, uint(id), req.PollIDs); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// MyPolls 我的投票（供关联选择）。
func (h *PollHandler) MyPolls(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	items, err := h.svc.ListMine(u.ID, 100)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}
