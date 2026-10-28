package handler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// AdminHandler 后台管理接口（阶段一：概览、用户、标签、投稿审核）。
type AdminHandler struct {
	svc    *service.AdminService
	subSvc *service.SubmissionService
}

func NewAdminHandler(svc *service.AdminService, subSvc *service.SubmissionService) *AdminHandler {
	return &AdminHandler{svc: svc, subSvc: subSvc}
}

func adminUser(c *gin.Context) *model.User {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		c.Abort()
		return nil
	}
	if !u.IsAdmin() {
		response.FailCode(c, errs.CodeForbidden, "")
		c.Abort()
		return nil
	}
	return u
}

// requireSuperAdmin 校验超级管理员身份（系统配置、日志与运维面板专用）。
func requireSuperAdmin(c *gin.Context) *model.User {
	u := adminUser(c)
	if u == nil {
		return nil
	}
	if !u.IsSuperAdmin() {
		response.FailCode(c, errs.CodeForbidden, "仅超级管理员可操作")
		c.Abort()
		return nil
	}
	return u
}

// Dashboard 概览。
func (h *AdminHandler) Dashboard(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	response.OK(c, h.svc.Dashboard())
}

// Users 用户列表。
func (h *AdminHandler) Users(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	list, total, err := h.svc.ListUsers(u, c.Query("keyword"), page, pageSize)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

// UserDetail 用户详情（仅后台）。返回含真实姓名/学校/联系方式等敏感字段；
// 这些字段在 User 模型上为 json:"-" 不随列表与公开接口泄露，故单独提供。
func (h *AdminHandler) UserDetail(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	u, err := h.svc.GetUser(uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{
		"id":             u.ID,
		"username":       u.Username,
		"email":          u.Email,
		"email_verified": u.EmailVerified,
		"role":           u.Role,
		"title":          u.Title,
		"status":         u.Status,
		"real_name":      u.RealName,
		"school":         u.School,
		"wechat":         u.Wechat,
		"qq":             u.QQ,
		"phone":          u.Phone,
		"created_at":     u.CreatedAt,
		"updated_at":     u.UpdatedAt,
	})
}

// SetRole 角色变更（仅超管）。
func (h *AdminHandler) SetRole(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.SetRole(u, uint(id), model.Role(req.Role)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// SetTitle 设置/修改/清除用户称号（仅超管；title 为空表示清除）。
func (h *AdminHandler) SetTitle(c *gin.Context) {
	u := requireSuperAdmin(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.SetTitle(u, uint(id), req.Title); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// PendingTags 待审核标签。
func (h *AdminHandler) PendingTags(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	tags, err := h.svc.PendingTags()
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": tags})
}

// ApproveTag 审核标签。
func (h *AdminHandler) ApproveTag(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Approve bool `json:"approve"`
	}
	c.ShouldBindJSON(&req)
	if err := h.svc.ApproveTag(u, uint(id), req.Approve); err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, nil)
}

// MergeTag 合并标签。
func (h *AdminHandler) MergeTag(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	var req struct {
		From uint `json:"from"`
		To   uint `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.MergeTag(u, req.From, req.To); err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, nil)
}

// AllTags 全部标签。
func (h *AdminHandler) AllTags(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	tags, err := h.svc.AllTags(c.Query("keyword"))
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": tags})
}

// CreateTag 手动新增标签。
func (h *AdminHandler) CreateTag(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	t, err := h.svc.CreateTag(u, req.Name)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, t)
}

// UpdateTag 修改标签名。
func (h *AdminHandler) UpdateTag(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.UpdateTag(u, uint(id), req.Name); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// DeleteTag 删除标签。
func (h *AdminHandler) DeleteTag(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteTag(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// PendingSubmissions 待审核投稿。
func (h *AdminHandler) PendingSubmissions(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	list, total, err := h.svc.PendingSubmissions(page, pageSize)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": h.submissionItems(list, u), "total": total, "page": page, "page_size": pageSize})
}

// AllSubmissions 全部稿件管理（支持状态筛选与关键词搜索）。
func (h *AdminHandler) AllSubmissions(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	status := c.Query("status")
	keyword := c.Query("keyword")
	list, total, err := h.svc.AllSubmissions(page, pageSize, status, keyword)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": h.submissionItems(list, u), "total": total, "page": page, "page_size": pageSize})
}

// submissionItems 将投稿列表转为视图，并附带机审（腾讯云 / AI）判定结果。
func (h *AdminHandler) submissionItems(list []model.Submission, u *model.User) []gin.H {
	bizIDs := make([]string, 0, len(list))
	for i := range list {
		bizIDs = append(bizIDs, fmt.Sprintf("sub-%d", list[i].ID))
	}
	auditMap, _ := h.svc.ModerationByBiz(bizIDs)
	items := make([]gin.H, 0, len(list))
	for i := range list {
		v := submissionView(&list[i], u, 0)
		if ts, ok := auditMap[fmt.Sprintf("sub-%d", list[i].ID)]; ok {
			aud := make([]gin.H, 0, len(ts))
			for _, t := range ts {
				reason := t.Label
				if t.Kind == "ai" {
					reason = t.RawResp // AI 返回的简短驳回原因
				} else if t.Label != "" {
					reason = "腾讯云判定：" + t.Label
				}
				aud = append(aud, gin.H{
					"kind":   t.Kind,
					"status": t.Status,
					"label":  t.Label,
					"reason": reason,
				})
			}
			v["audit"] = aud
		}
		items = append(items, v)
	}
	return items
}

// RestoreSubmission 恢复被撤下的投稿。
func (h *AdminHandler) RestoreSubmission(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.subSvc.Restore(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// ReviewSubmission 审核投稿。
func (h *AdminHandler) ReviewSubmission(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	var status model.SubmissionStatus
	switch req.Status {
	case "approved", "pending", "rejected", "need_edit":
		status = model.SubmissionStatus(req.Status)
	default:
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.subSvc.Review(u, uint(id), status, req.Note); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// PinSubmission 置顶投稿（管理员）。可附带 order（排序，越小越靠前）与可选有效期 expires_at。
func (h *AdminHandler) PinSubmission(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailCode(c, errs.ErrValidation, "无效的投稿 ID")
		return
	}
	var req struct {
		Order     int    `json:"order"`
		ExpiresAt string `json:"expires_at"` // 可选；格式 "2006-01-02 15:04:05" 或 RFC3339
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, perr := time.Parse("2006-01-02 15:04:05", req.ExpiresAt)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, req.ExpiresAt)
		}
		if perr != nil {
			response.FailCode(c, errs.ErrValidation, "有效期格式错误，应为 2006-01-02 15:04:05")
			return
		}
		expiresAt = &t
	}
	if err := h.subSvc.PinSubmission(u, uint(id), req.Order, expiresAt); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// UnpinSubmission 取消置顶（管理员）。
func (h *AdminHandler) UnpinSubmission(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailCode(c, errs.ErrValidation, "无效的投稿 ID")
		return
	}
	if err := h.subSvc.UnpinSubmission(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
