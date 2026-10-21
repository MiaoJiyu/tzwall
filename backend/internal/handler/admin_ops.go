package handler

import (
	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// OpsHandler 运维面板（仅超级管理员）。
type OpsHandler struct {
	svc *service.OpsService
}

func NewOpsHandler(svc *service.OpsService) *OpsHandler {
	return &OpsHandler{svc: svc}
}

// Overview 聚合磁盘、数据库、备份、健康与 COS 用量。
func (h *OpsHandler) Overview(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}
	o, err := h.svc.Overview(c.Request.Context())
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, o)
}
