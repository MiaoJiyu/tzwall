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

// TicketHandler 工单接口（用户端 + 后台管理）。
type TicketHandler struct {
	svc *service.TicketService
}

func NewTicketHandler(svc *service.TicketService) *TicketHandler {
	return &TicketHandler{svc: svc}
}

// Create 提交工单（需登录）。
func (h *TicketHandler) Create(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var req service.CreateTicketReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	t, err := h.svc.Create(u, req)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, t)
}

// ListMine 我的工单（需登录）。
func (h *TicketHandler) ListMine(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	list, err := h.svc.ListMine(u, c.Query("status"))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": list})
}

// Get 工单详情（作者或管理员）。
func (h *TicketHandler) Get(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := h.svc.Get(u, uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, t)
}

// Close 用户关闭自己的工单。
func (h *TicketHandler) Close(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.CloseOwn(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminList 后台工单列表（管理员）。
func (h *TicketHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	list, total, err := h.svc.AdminList(page, pageSize, c.Query("status"), c.Query("type"), c.Query("keyword"))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

// AdminGet 后台工单详情（管理员）。
func (h *TicketHandler) AdminGet(c *gin.Context) {
	admin := adminUser(c)
	if admin == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := h.svc.Get(admin, uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, t)
}

// Reply 管理员回复工单。
func (h *TicketHandler) Reply(c *gin.Context) {
	admin := adminUser(c)
	if admin == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Reply(admin, uint(id), req.Content); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// SetStatus 管理员流转工单状态。
func (h *TicketHandler) SetStatus(c *gin.Context) {
	admin := adminUser(c)
	if admin == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.SetStatus(admin, uint(id), model.TicketStatus(req.Status)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
