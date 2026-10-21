package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// AgreementHandler 协议管理：当前生效内容、历史版本、发布与回滚（仅超级管理员）。
type AgreementHandler struct {
	svc *service.AgreementService
}

func NewAgreementHandler(svc *service.AgreementService) *AgreementHandler {
	return &AgreementHandler{svc: svc}
}

// List 当前协议 + 全部历史版本。
func (h *AgreementHandler) List(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}
	current, err := h.svc.Current()
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	type group struct {
		Key      string                   `json:"key"`
		Current  *model.Agreement         `json:"current"`
		Versions []model.AgreementVersion `json:"versions"`
	}
	groups := make([]group, 0, len(current))
	for i := range current {
		versions, _ := h.svc.Versions(current[i].Key)
		cur := current[i]
		groups = append(groups, group{Key: cur.Key, Current: &cur, Versions: versions})
	}
	response.OK(c, gin.H{"items": groups})
}

// Publish 发布新版本。
func (h *AgreementHandler) Publish(c *gin.Context) {
	u := requireSuperAdmin(c)
	if u == nil {
		return
	}
	key := c.Param("key")
	var req struct {
		Title       string `json:"title"`
		Version     string `json:"version"`
		Content     string `json:"content"`
		EffectiveAt string `json:"effective_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	var effective time.Time
	if req.EffectiveAt != "" {
		if t, err := time.Parse("2006-01-02", req.EffectiveAt); err == nil {
			effective = t
		}
	}
	if err := h.svc.Publish(u, key, service.PublishReq{
		Title:       req.Title,
		Version:     req.Version,
		Content:     req.Content,
		EffectiveAt: effective,
	}); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Rollback 回滚到指定历史版本。
func (h *AgreementHandler) Rollback(c *gin.Context) {
	u := requireSuperAdmin(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Rollback(u, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
