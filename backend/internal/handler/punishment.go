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

// PunishmentHandler 处罚相关接口。
type PunishmentHandler struct {
	svc *service.PunishmentService
}

func NewPunishmentHandler(svc *service.PunishmentService) *PunishmentHandler {
	return &PunishmentHandler{svc: svc}
}

// Status 当前用户处罚状态（警告/禁言/强制改密），前端据此弹窗或置灰。
func (h *PunishmentHandler) Status(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.OK(c, service.Snapshot{})
		return
	}
	response.OK(c, h.svc.Snapshot(u.ID))
}

// Mine 我的权限变更记录。
func (h *PunishmentHandler) Mine(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	rows, err := h.svc.ListMine(u.ID)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// Ack 确认警告（确认后不再弹窗）。
func (h *PunishmentHandler) Ack(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Ack(u.ID, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminCreate 创建处罚（管理员）。
func (h *PunishmentHandler) AdminCreate(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	var req struct {
		UserID      uint                 `json:"user_id"`
		Type        model.PunishmentType `json:"type"`
		Reason      string               `json:"reason"`
		Permanent   bool                 `json:"permanent"`
		ExpiresAt   *string              `json:"expires_at"`
		TargetIP    string               `json:"target_ip"`
		NewUsername string               `json:"new_username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	p, tempPwd, err := h.svc.Create(actor, service.PunishRequest{
		UserID:      req.UserID,
		Type:        req.Type,
		Reason:      req.Reason,
		Permanent:   req.Permanent,
		ExpiresAt:   parseTimePtr(req.ExpiresAt),
		TargetIP:    req.TargetIP,
		NewUsername: req.NewUsername,
	})
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"id": p.ID, "temp_password": tempPwd})
}

// AdminList 处罚列表（管理员）。
func (h *PunishmentHandler) AdminList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	userID := uint(atoiDefault(c.Query("user_id"), 0))
	rows, total, err := h.svc.ListForAdmin(userID, page, pageSize)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize})
}

// AdminRevoke 撤销处罚（仅标记，不恢复强制改密前的用户名/密码）。
func (h *PunishmentHandler) AdminRevoke(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Revoke(actor, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// IPBanCreate 封禁 IP（支持 IPv4/IPv6/CIDR）。
func (h *PunishmentHandler) IPBanCreate(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	var req struct {
		CIDR      string  `json:"cidr"`
		Reason    string  `json:"reason"`
		Permanent bool    `json:"permanent"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if req.CIDR == "" || req.Reason == "" {
		response.FailCode(c, errs.ErrValidation, "IP 与理由均为必填")
		return
	}
	p, _, err := h.svc.Create(actor, service.PunishRequest{
		Type:      model.PunishBanIP,
		Reason:    req.Reason,
		TargetIP:  req.CIDR,
		Permanent: req.Permanent,
		ExpiresAt: parseTimePtr(req.ExpiresAt),
	})
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"id": p.ID})
}

// IPBanList IP 封禁列表。
func (h *PunishmentHandler) IPBanList(c *gin.Context) {
	if adminUser(c) == nil {
		return
	}
	rows, err := h.svc.ListIPBans()
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// IPBanRevoke 撤销 IP 封禁。
func (h *PunishmentHandler) IPBanRevoke(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.RevokeIPBan(actor, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
