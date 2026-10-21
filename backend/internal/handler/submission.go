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

// SubmissionHandler 投稿接口。
type SubmissionHandler struct {
	svc *service.SubmissionService
}

func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

// submissionView 转为前端视图，处理匿名、联系方式可见性与图片代理地址。
func submissionView(sub *model.Submission, viewer *model.User) gin.H {
	author := "匿名用户"
	realAuthor := ""
	showContact := false
	if !sub.Anonymous {
		if sub.User != nil {
			author = sub.User.Username
			realAuthor = sub.User.Username
		}
		if sub.ContactVisible == model.ContactAll {
			showContact = true
		} else if sub.ContactVisible == model.ContactLogin && viewer != nil {
			showContact = true
		}
	}
	if viewer != nil && viewer.IsAdmin() {
		showContact = true // 管理员始终可见完整联系方式
	}
	contact := ""
	if showContact {
		contact = sub.Contact
	}

	viewerUID := 0
	if viewer != nil {
		viewerUID = int(viewer.ID)
	}
	imgs := make([]gin.H, 0, len(sub.Images))
	for _, im := range sub.Images {
		imgs = append(imgs, gin.H{
			"id":           im.ID,
			"url":          imageURL(im, viewerUID),
			"content_type": im.ContentType,
			"width":        im.Width,
			"height":       im.Height,
		})
	}
	tags := make([]gin.H, 0, len(sub.Tags))
	for _, t := range sub.Tags {
		tags = append(tags, gin.H{"id": t.ID, "name": t.Name})
	}
	vids := make([]gin.H, 0, len(sub.Videos))
	for _, vd := range sub.Videos {
		vids = append(vids, gin.H{
			"id":           vd.ID,
			"url":          videoURL(vd, viewerUID),
			"content_type": vd.ContentType,
			"size":         vd.Size,
		})
	}

	v := gin.H{
		"id":              sub.ID,
		"title":           sub.Title,
		"content":         sub.Content,
		"anonymous":       sub.Anonymous,
		"author":          author,
		"status":          sub.Status,
		"admin_note":      sub.AdminNote,
		"contact":         contact,
		"contact_visible": sub.ContactVisible,
		"like_count":      sub.LikeCount,
		"dislike_count":   sub.DislikeCount,
		"comment_count":   sub.CommentCount,
		"view_count":      sub.ViewCount,
		"revision_count":  sub.RevisionCount,
		"tags":            tags,
		"images":          imgs,
		"videos":          vids,
		"created_at":      sub.CreatedAt,
		"user_id":         sub.UserID,
	}
	if viewer != nil && viewer.IsAdmin() && realAuthor != "" {
		v["real_author"] = realAuthor
	}
	// AI 审核评论（通过/拒绝原因），仅管理员在后台可见
	if viewer != nil && viewer.IsAdmin() {
		v["audit_comment"] = sub.AuditComment
	}
	return v
}

// Create 发布投稿。
func (h *SubmissionHandler) Create(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var req service.CreateSubmissionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	sub, err := h.svc.Create(u, req)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, submissionView(sub, u))
}

// Update 编辑重提。
func (h *SubmissionHandler) Update(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req service.CreateSubmissionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	sub, err := h.svc.Update(u, uint(id), req)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, submissionView(sub, u))
}

// Delete 删除投稿。
func (h *SubmissionHandler) Delete(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// List 公开列表。
func (h *SubmissionHandler) List(c *gin.Context) {
	p := service.ListParams{
		Sort:     c.Query("sort"),
		Tag:      c.Query("tag"),
		Keyword:  c.Query("keyword"),
		Page:     atoiDefault(c.Query("page"), 1),
		PageSize: atoiDefault(c.Query("page_size"), 20),
	}
	subs, total, err := h.svc.List(p)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	viewer := middleware.CurrentUser(c)
	items := make([]gin.H, 0, len(subs))
	for i := range subs {
		items = append(items, submissionView(&subs[i], viewer))
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": p.Page, "page_size": p.PageSize})
}

// Get 详情（可见性控制）。
func (h *SubmissionHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	viewer := middleware.CurrentUser(c)
	sub, err := h.svc.GetVisible(uint(id), viewer)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	if sub.Status == model.StatusApproved {
		h.svc.IncrView(uint(id))
	}
	response.OK(c, submissionView(sub, viewer))
}

// ListMine 我的投稿。
func (h *SubmissionHandler) ListMine(c *gin.Context) {
	u := middleware.CurrentUser(c)
	subs, err := h.svc.ListMine(u)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	items := make([]gin.H, 0, len(subs))
	for i := range subs {
		items = append(items, submissionView(&subs[i], u))
	}
	response.OK(c, gin.H{"items": items})
}

// Tags 已审核标签列表。
func (h *SubmissionHandler) Tags(c *gin.Context) {
	tags, err := h.svc.ApprovedTags()
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": tags})
}
