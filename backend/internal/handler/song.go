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

// SongHandler 点歌接口。
type SongHandler struct {
	svc *service.SongService
}

func NewSongHandler(svc *service.SongService) *SongHandler {
	return &SongHandler{svc: svc}
}

// List 公开点歌列表。
func (h *SongHandler) List(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	songs, total, err := h.svc.List(page, pageSize, c.Query("status"))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": songs, "total": total, "page": page, "page_size": pageSize})
}

// Quota 本周点歌配额（需登录）。
func (h *SongHandler) Quota(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	response.OK(c, h.svc.Quota(u.ID))
}

// Create 提交点歌（需登录）。
func (h *SongHandler) Create(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var req struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	song, err := h.svc.Create(u, req.Title, req.Artist, req.Note)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, song)
}

// AdminList 后台点歌列表。
func (h *SongHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	songs, total, err := h.svc.AdminList(page, pageSize, c.Query("status"), c.Query("keyword"))
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": songs, "total": total, "page": page, "page_size": pageSize})
}

// SetStatus 状态流转（待播放/已播放/已取消）。
func (h *SongHandler) SetStatus(c *gin.Context) {
	u := adminUser(c)
	if u == nil {
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
	if err := h.svc.SetStatus(u, uint(id), model.SongStatus(req.Status)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminDelete 删除点歌记录。
func (h *SongHandler) AdminDelete(c *gin.Context) {
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
