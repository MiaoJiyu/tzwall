package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// CommentHandler 评论接口。
type CommentHandler struct {
	svc *service.CommentService
}

func NewCommentHandler(svc *service.CommentService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

// List 评论树。管理员可见已删除评论原文。
func (h *CommentHandler) List(c *gin.Context) {
	u := middleware.CurrentUser(c)
	isAdmin := u != nil && u.IsAdmin()
	viewerID := uint(0)
	if u != nil {
		viewerID = u.ID
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	roots, err := h.svc.List(uint(id), viewerID, isAdmin)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": roots})
}

// Like 点赞/取消点赞评论（幂等切换；未登录由路由层拦截）。
func (h *CommentHandler) Like(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	liked, count, err := h.svc.ToggleLike(u.ID, uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"liked": liked, "like_count": count})
}

// AdminList 后台评论管理列表（关键词/状态/投稿/作者筛选 + 分页）。
func (h *CommentHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	var subID uint
	if v, err := strconv.ParseUint(c.Query("submission_id"), 10, 64); err == nil {
		subID = uint(v)
	}
	rows, total, err := h.svc.AdminList(service.AdminCommentQuery{
		Keyword:      c.Query("keyword"),
		Status:       c.Query("status"),
		SubmissionID: subID,
		Author:       c.Query("author"),
		Page:         atoiDefault(c.Query("page"), 1),
		PageSize:     atoiDefault(c.Query("page_size"), 20),
	})
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total, "page": atoiDefault(c.Query("page"), 1), "page_size": atoiDefault(c.Query("page_size"), 20)})
}

// AdminRestore 管理员恢复撤下的评论（记录操作日志）。
func (h *CommentHandler) AdminRestore(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.AdminRestore(actor.ID, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminRemove 管理员撤下评论（记录操作日志）。
func (h *CommentHandler) AdminRemove(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.AdminRemove(actor.ID, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Create 发表评论。
func (h *CommentHandler) Create(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		ParentID uint   `json:"parent_id"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	cm, err := h.svc.Create(u, uint(id), req.ParentID, req.Content)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, cm)
}

// Delete 删除自己的评论。
func (h *CommentHandler) Delete(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
