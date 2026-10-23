package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// UserHandler 用户公开资料。
type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Public 用户公开主页（无需登录；未开启公开返回 403）。
// 历史发帖复用统一 submissionView，确保点赞/评论/浏览计数、作者、正文、图片等字段齐全。
func (h *UserHandler) Public(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	p, subs, err := h.svc.GetPublic(uint(id))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	viewer := middleware.CurrentUser(c)
	posts := make([]gin.H, 0, len(subs))
	for i := range subs {
		posts = append(posts, submissionView(&subs[i], viewer, 0))
	}
	response.OK(c, gin.H{
		"id":       p.ID,
		"username": p.Username,
		"nickname": p.Nickname,
		"avatar":   p.Avatar,
		"title":    p.Title,
		"fields":   p.Fields,
		"posts":    posts,
	})
}
