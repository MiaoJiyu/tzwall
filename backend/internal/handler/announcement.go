package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// AnnouncementHandler 校园公告（公开列表/确认 + 后台管理）。
type AnnouncementHandler struct {
	svc *service.AnnouncementService
}

func NewAnnouncementHandler(svc *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{svc: svc}
}

// List 公开公告列表（含当前查看者确认状态）。
func (h *AnnouncementHandler) List(c *gin.Context) {
	uid := uint(0)
	if u := middleware.CurrentUser(c); u != nil {
		uid = u.ID
	}
	views, err := h.svc.ListPublic(uid)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": views})
}

// Confirm 用户确认公告（需登录）。
func (h *AnnouncementHandler) Confirm(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Confirm(u.ID, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminList 后台公告列表。
func (h *AnnouncementHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	list, err := h.svc.AdminList()
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": list})
}

// Create 发布公告。
func (h *AnnouncementHandler) Create(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	var req struct {
		Title          string `json:"title"`
		Content        string `json:"content"`
		RequireConfirm bool   `json:"require_confirm"`
		Pinned         bool   `json:"pinned"`
		Active         bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	a, err := h.svc.Create(u, req.Title, req.Content, req.RequireConfirm, req.Pinned, req.Active)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"id": a.ID})
}

// Update 修改公告。
func (h *AnnouncementHandler) Update(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Title          string `json:"title"`
		Content        string `json:"content"`
		RequireConfirm bool   `json:"require_confirm"`
		Pinned         bool   `json:"pinned"`
		Active         bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Update(u, uint(id), req.Title, req.Content, req.RequireConfirm, req.Pinned, req.Active); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Delete 删除公告。
func (h *AnnouncementHandler) Delete(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
