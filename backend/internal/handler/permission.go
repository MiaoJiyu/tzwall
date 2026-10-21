package handler

import (
	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/permission"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// PermissionHandler 权限矩阵接口（仅超级管理员）。
type PermissionHandler struct {
	svc *service.PermissionService
	res *permission.Resolver
}

func NewPermissionHandler(svc *service.PermissionService, res *permission.Resolver) *PermissionHandler {
	return &PermissionHandler{svc: svc, res: res}
}

func superUser(c *gin.Context) *model.User {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		c.Abort()
		return nil
	}
	if !u.IsSuperAdmin() {
		response.FailCode(c, errs.CodeForbidden, "")
		c.Abort()
		return nil
	}
	return u
}

// Endpoints 端点目录与页面清单（供后台矩阵展示）。
func (h *PermissionHandler) Endpoints(c *gin.Context) {
	if superUser(c) == nil {
		return
	}
	response.OK(c, gin.H{
		"endpoints": permission.Endpoints,
		"pages":     permission.Pages,
	})
}

// Matrix 读取某角色或某用户的权限矩阵。
func (h *PermissionHandler) Matrix(c *gin.Context) {
	if superUser(c) == nil {
		return
	}
	role := c.Query("role")
	userID := uint(atoiDefault(c.Query("user_id"), 0))
	if role == "" && userID == 0 {
		role = string(model.RoleUser)
	}
	items, err := h.svc.Matrix(role, userID)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"role": role, "user_id": userID, "items": items})
}

// Save 保存权限矩阵。
func (h *PermissionHandler) Save(c *gin.Context) {
	u := superUser(c)
	if u == nil {
		return
	}
	var req struct {
		Role   string               `json:"role"`
		UserID uint                 `json:"user_id"`
		Items  []service.GrantInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Save(u, req.Role, req.UserID, req.Items); err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, nil)
}
